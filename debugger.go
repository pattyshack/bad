package bad

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"strconv"
	"syscall"

	"golang.org/x/arch/x86/x86asm"

	"github.com/pattyshack/bad/ptrace"
)

const (
	maxX64InstructionLength = 15

	syscallTrapSignal         = syscall.SIGTRAP | 0x80
	waitStatusSyscallTrapMask = ^uint32(0x80 << 8)
)

var (
	userDebugRegistersOffset = uintptr(0) // initialized by init()

	ErrProcessExited = fmt.Errorf("process exited")
)

type VirtualAddress uint64

func (addr VirtualAddress) String() string {
	return fmt.Sprintf("0x%016x", uint64(addr))
}

func ParseVirtualAddress(value string) (VirtualAddress, error) {
	addr, err := strconv.ParseUint(value, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse virtual address (%s): %w", value, err)
	}

	return VirtualAddress(addr), nil
}

type DisassembledInstruction struct {
	Address VirtualAddress
	x86asm.Inst
}

func (inst DisassembledInstruction) String() string {
	return fmt.Sprintf(
		"0x%016x: %s",
		uint64(inst.Address),
		x86asm.GNUSyntax(inst.Inst, uint64(inst.Address), nil))
}

type Debugger struct {
	tracer *ptrace.Tracer

	*RegisterSet

	hardwareStopPoints *hardwareStopPointAllocator

	BreakPointSites *StopPointSet
	WatchPoints     *StopPointSet

	SyscallCatchPolicy *SyscallCatchPolicy

	Pid         int
	ownsProcess bool

	state              ProcessState
	expectsSyscallExit bool

	ctx    context.Context
	cancel func()

	sigIntChan chan os.Signal
}

func newDebugger(tracer *ptrace.Tracer, ownsProcess bool) (*Debugger, error) {
	hwStopPoints := newHardwareStopPointAllocator()
	allocator := NewStopPointAllocator(hwStopPoints)

	ctx, cancel := context.WithCancel(context.Background())

	db := &Debugger{
		tracer:             tracer,
		RegisterSet:        NewRegisterSet(),
		hardwareStopPoints: hwStopPoints,
		BreakPointSites:    NewBreakPointSites(allocator),
		WatchPoints:        NewWatchPoints(allocator),
		SyscallCatchPolicy: NewSyscallCatchPolicy(),
		Pid:                tracer.Pid(),
		ownsProcess:        ownsProcess,
		state:              newRunningProcessState(tracer.Pid()),
		ctx:                ctx,
		cancel:             cancel,
		sigIntChan:         make(chan os.Signal),
	}

	allocator.SetDebugger(db)

	_, err := db.waitForSignal()
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	if ownsProcess {
		err = tracer.SetOptions(ptrace.O_EXITKILL | ptrace.O_TRACESYSGOOD)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf(
				"failed to set PTRACE_O_EXITKILL on process %d",
				tracer.Pid())
		}
	}

	signal.Notify(db.sigIntChan, os.Interrupt)
	go db.processSigInt()

	return db, nil
}

func AttachTo(pid int) (*Debugger, error) {
	tracer, err := ptrace.AttachToProcess(pid)
	if err != nil {
		return nil, err
	}

	return newDebugger(tracer, false)
}

func StartAndAttachTo(cmd *exec.Cmd) (*Debugger, error) {
	tracer, err := ptrace.StartAndAttachToProcess(cmd)
	if err != nil {
		return nil, err
	}

	return newDebugger(tracer, true)
}

func StartCmdAndAttachTo(name string, args ...string) (*Debugger, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return StartAndAttachTo(cmd)
}

func (db *Debugger) processSigInt() {
	for {
		select {
		case <-db.ctx.Done():
			return
		case <-db.sigIntChan:
			err := db.signal(syscall.SIGSTOP)
			if err != nil {
				panic(err)
			}
		}
	}
}

func (db *Debugger) State() ProcessState {
	return db.state
}

func (db *Debugger) resume() error {
	if db.state.Exited() {
		return fmt.Errorf(
			"failed to resume process %d: %w",
			db.Pid,
			ErrProcessExited)
	}

	site, ok := db.BreakPointSites.Get(db.state.NextInstructionAddress)
	if ok && site.IsEnabled() {
		_, err := db.stepInstruction()
		if err != nil {
			return fmt.Errorf("failed to resume process %d: %w", db.Pid, err)
		}
	}

	var err error
	if db.SyscallCatchPolicy.IsEnabled() {
		err = db.tracer.SyscallTrappedResume(0)
	} else {
		err = db.tracer.Resume(0)
	}
	if err != nil {
		return fmt.Errorf("failed to resume process %d: %w", db.Pid, err)
	}

	db.state = newRunningProcessState(db.Pid)
	return nil
}

func (db *Debugger) ResumeUntilSignal() (ProcessState, error) {
	err := db.resume()
	if err != nil {
		return ProcessState{}, err
	}

	state, err := db.waitForSignal()
	if err != nil {
		return ProcessState{}, fmt.Errorf(
			"failed to resume process %d: %w",
			db.Pid,
			err)
	}

	return state, nil
}

func (db *Debugger) StepInstruction() (ProcessState, error) {
	if db.state.Exited() {
		return db.state, fmt.Errorf(
			"failed to step instruction for process %d: %w",
			db.Pid,
			ErrProcessExited)
	}

	state, err := db.stepInstruction()
	if err != nil {
		return state, fmt.Errorf(
			"failed to step instruction for process %d: %w",
			db.Pid,
			err)
	}

	return state, nil
}

func (db *Debugger) stepInstruction() (ProcessState, error) {
	addr := db.state.NextInstructionAddress

	site, ok := db.BreakPointSites.Get(addr)
	siteIsEnabled := ok && site.IsEnabled()
	if siteIsEnabled {
		err := site.Disable()
		if err != nil {
			return ProcessState{}, fmt.Errorf(
				"failed to disable break point at %s: %w",
				addr,
				err)
		}
	}

	err := db.tracer.SingleStep()
	if err != nil {
		return ProcessState{}, fmt.Errorf(
			"failed to single step at %s: %w",
			addr,
			err)
	}

	state, err := db.waitForSignal()
	if err != nil {
		return ProcessState{}, fmt.Errorf(
			"failed to wait for step instruction at %s: %w",
			addr,
			err)
	}

	if siteIsEnabled {
		err = site.Enable()
		if err != nil {
			return ProcessState{}, fmt.Errorf(
				"failed to re-enable break point at %s: %w",
				addr,
				err)
		}
	}

	return state, nil
}

// NOTE: this creates a new ProcessState without making any modification to
// the debugger's internal state.
func (db *Debugger) newProcessState(
	status syscall.WaitStatus,
) (
	ProcessState,
	error,
) {
	state := ProcessState{
		Pid:    db.Pid,
		Status: &status,
	}

	if !status.Stopped() {
		return state, nil
	}

	registerState, pc, err := db.getProgramCounter()
	if err != nil {
		return ProcessState{}, err
	}
	state.NextInstructionAddress = pc

	if status.StopSignal() == syscallTrapSignal {
		state.TrapReason = SyscallTrap

		// Replaced the modified syscall trap signal with a normal trap signal.
		trapStatus := syscall.WaitStatus(uint32(status) & waitStatusSyscallTrapMask)
		state.Status = &trapStatus

		reg, ok := db.RegisterByName("orig_rax")
		if !ok {
			panic("should never happen")
		}

		sysNum := int(registerState.Value(reg).ToUint32())
		id, ok := GetSyscallIdByNumber(sysNum)
		if !ok {
			return ProcessState{}, fmt.Errorf(
				"trapped unknown syscall number (%d)",
				sysNum)
		}

		info := &SyscallTrapInfo{
			IsEntry: !db.expectsSyscallExit,
			Id:      id,
		}
		state.SyscallTrapInfo = info

		if db.expectsSyscallExit { // syscall returned
			reg, ok = db.RegisterByName("rax")
			if !ok {
				panic("should never happen")
			}

			info.Ret = registerState.Value(reg).ToUint64()
		} else { // syscall entry
			for idx, arg := range []string{"rdi", "rsi", "rdx", "r10", "r8", "r9"} {
				reg, ok = db.RegisterByName(arg)
				if !ok {
					panic("should never happen")
				}

				info.Args[idx] = registerState.Value(reg).ToUint64()
			}
		}

		return state, nil
	}

	if status.StopSignal() != syscall.SIGTRAP {
		return state, nil
	}

	sigInfo, err := db.tracer.GetSigInfo()
	if err != nil {
		return ProcessState{}, err
	}

	state.TrapReason = TrapCodeToReason(sigInfo.Code)
	return state, nil
}

func (db *Debugger) waitForSignal() (ProcessState, error) {
	for {
		state, err := db.waitForAnySignal()
		if err != nil {
			return ProcessState{}, err
		}

		if !state.Stopped() ||
			state.StopSignal() != syscall.SIGTRAP ||
			state.TrapReason != SyscallTrap ||
			db.SyscallCatchPolicy.Matches(state.SyscallTrapInfo.Id) {

			return state, err
		}

		err = db.resume()
		if err != nil {
			return ProcessState{}, err
		}
	}
}

// NOTE: This returns on all traps, including traps on syscall that we don't
// care about.
func (db *Debugger) waitForAnySignal() (ProcessState, error) {
	// NOTE: golang does not support waitpid
	var status syscall.WaitStatus
	_, err := syscall.Wait4(db.Pid, &status, 0, nil)
	if err != nil {
		return ProcessState{}, fmt.Errorf(
			"failed to wait for process %d: %w",
			db.Pid,
			err)
	}

	state, err := db.newProcessState(status)
	if err != nil {
		return ProcessState{}, fmt.Errorf(
			"failed to wait for process %d: %w",
			db.Pid,
			err)
	}

	db.state = state

	if !db.state.Stopped() || db.state.StopSignal() != syscall.SIGTRAP {
		return db.state, nil
	}

	if db.state.TrapReason == SyscallTrap {
		db.expectsSyscallExit = !db.expectsSyscallExit
		return db.state, nil
	}

	// In case syscall catch point got disabled after syscall entry, but before
	// syscall exit.
	db.expectsSyscallExit = false

	if db.state.TrapReason == SoftwareTrap {
		// NOTE: currentInstructionAddr may not be a valid instruction address
		// since x64 instruction could span multiple bytes.  However, since
		// break point sites are implemented using the int3 (0xcc), we know for
		// sure the address is valid if the current instruction is a break
		// point site.
		currentInstructionAddr := db.state.NextInstructionAddress - 1

		site, ok := db.BreakPointSites.Get(currentInstructionAddr)
		if ok && site.IsEnabled() {
			// set pc back to int3's address
			err := db.setProgramCounter(currentInstructionAddr)
			if err != nil {
				return ProcessState{}, fmt.Errorf(
					"failed to wait for process %d. "+
						"cannot reset program counter at break point: %w",
					db.Pid,
					err)
			}

			db.state.StopPoints = append(db.state.StopPoints, site)
		}
	} else if db.state.TrapReason == HardwareTrap {
		triggered, err := db.hardwareStopPoints.ListTriggered()
		if err != nil {
			return ProcessState{}, fmt.Errorf(
				"failed to wait for process %d: %w",
				db.Pid,
				err)
		}

		db.state.StopPoints = append(db.state.StopPoints, triggered...)
	}

	return db.state, nil
}

func (db *Debugger) signal(signal syscall.Signal) error {
	err := syscall.Kill(db.Pid, signal)
	if err != nil {
		return fmt.Errorf("failed to signal to process %d (%v): %w",
			db.Pid,
			signal,
			err)
	}

	return nil
}

func (db *Debugger) Close() error {
	db.cancel()

	if db.state.Running() {
		err := db.signal(syscall.SIGSTOP)
		if err != nil {
			return err
		}

		_, err = db.waitForSignal()
		if err != nil {
			return err
		}
	}

	if db.state.Exited() { // nothing to detach from
		return nil
	}

	err := db.tracer.Detach()
	if err != nil {
		return err
	}

	err = db.signal(syscall.SIGCONT)
	if err != nil {
		return err
	}

	if db.ownsProcess {
		err = db.signal(syscall.SIGKILL)
		if err != nil {
			return err
		}

		_, err = db.waitForSignal()
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *Debugger) GetRegisterState() (RegisterState, error) {
	if !db.state.Stopped() {
		return RegisterState{}, fmt.Errorf(
			"cannot get register state. process %d not stopped (%s)",
			db.Pid,
			db.state)
	}

	return db.getRegisterState()
}

func (db *Debugger) getRegisterState() (RegisterState, error) {
	gpr, err := db.tracer.GetGeneralRegisters()
	if err != nil {
		return RegisterState{}, err
	}

	fpr, err := db.tracer.GetFloatingPointRegisters()
	if err != nil {
		return RegisterState{}, err
	}

	regs := RegisterState{
		gpr: *gpr,
		fpr: *fpr,
	}

	for idx, _ := range regs.dr {
		offset := userDebugRegistersOffset + uintptr(idx*8)
		value, err := db.tracer.PeekUserArea(offset)
		if err != nil {
			return RegisterState{}, err
		}
		regs.dr[idx] = value
	}

	return regs, nil
}

func (db *Debugger) SetRegisterState(state RegisterState) error {
	if !db.state.Stopped() {
		return fmt.Errorf(
			"cannot set register state. process %d not stopped (%s)",
			db.Pid,
			db.state)
	}

	return db.setRegisterState(state)
}

func (db *Debugger) setRegisterState(state RegisterState) error {
	err := db.tracer.SetGeneralRegisters(&state.gpr)
	if err != nil {
		return err
	}

	err = db.tracer.SetFloatingPointRegisters(&state.fpr)
	if err != nil {
		return err
	}

	for idx, value := range state.dr {
		// dr4 and dr5 are not real registers
		// https://en.wikipedia.org/wiki/X86_debug_register
		if idx == 4 || idx == 5 {
			continue
		}

		offset := userDebugRegistersOffset + uintptr(idx*8)
		err := db.tracer.PokeUserArea(offset, value)
		if err != nil {
			return fmt.Errorf("failed to set dr%d: %w", idx, err)
		}
	}

	return nil
}

func (db *Debugger) getProgramCounter() (RegisterState, VirtualAddress, error) {
	state, err := db.getRegisterState()
	if err != nil {
		return RegisterState{}, 0, fmt.Errorf(
			"failed to read program counter: %w",
			err)
	}

	rip, ok := db.RegisterByName("rip")
	if !ok {
		panic("should never happen")
	}

	return state, VirtualAddress(state.Value(rip).ToUint64()), nil
}

func (db *Debugger) setProgramCounter(addr VirtualAddress) error {
	state, err := db.getRegisterState()
	if err != nil {
		return fmt.Errorf("failed to read program counter: %w", err)
	}

	rip, ok := db.RegisterByName("rip")
	if !ok {
		panic("should never happen")
	}

	newState, err := state.WithValue(rip, Uint64Value(uint64(addr)))
	if err != nil {
		return fmt.Errorf(
			"failed to update program counter state to %s: %w",
			addr,
			err)
	}

	err = db.setRegisterState(newState)
	if err != nil {
		return fmt.Errorf("failed to set program counter to %s: %w", addr, err)
	}

	db.state.NextInstructionAddress = addr
	return nil
}

func (db *Debugger) ReadFromVirtualMemory(
	addr VirtualAddress,
	out []byte,
) (
	int,
	error,
) {
	if !db.state.Stopped() {
		return 0, fmt.Errorf(
			"cannot read from virtual memory at %s. process %d not stopped (%s)",
			addr,
			db.Pid,
			db.state)
	}

	count, err := db.tracer.ReadFromVirtualMemory(uintptr(addr), out)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to read from virtual memory at %s (%d) for process %d: %w",
			addr,
			len(out),
			db.Pid,
			err)
	}

	return count, nil
}

func (db *Debugger) WriteToVirtualMemory(
	addr VirtualAddress,
	data []byte,
) (
	int,
	error,
) {
	if !db.state.Stopped() {
		return 0, fmt.Errorf(
			"cannot write to virtual memory at %s. process %d not stopped (%s)",
			addr,
			db.Pid,
			db.state)
	}

	count, err := db.tracer.PokeData(uintptr(addr), data)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to write to virtual memory at %s (%d) for process %d: %w",
			addr,
			len(data),
			db.Pid,
			err)
	}

	return count, nil
}

func (db *Debugger) Disassemble(
	startAddress VirtualAddress,
	numInstructions int,
) (
	[]DisassembledInstruction,
	error,
) {
	if numInstructions < 0 {
		return nil, fmt.Errorf(
			"Invalid number of instructions to disassemble: %d",
			numInstructions)
	} else if numInstructions == 0 {
		return nil, nil
	}

	data := make([]byte, numInstructions*maxX64InstructionLength)
	_, err := db.ReadFromVirtualMemory(startAddress, data)
	if err != nil {
		return nil, err
	}

	db.BreakPointSites.ReplaceStopPointBytes(startAddress, data)

	address := startAddress
	result := make([]DisassembledInstruction, 0, numInstructions)
	for len(data) > 0 && len(result) < numInstructions {
		inst, err := x86asm.Decode(data, 64)
		if err != nil {
			break
		}

		result = append(
			result,
			DisassembledInstruction{
				Address: address,
				Inst:    inst,
			})

		data = data[inst.Len:]
		address += VirtualAddress(inst.Len)
	}

	return result, nil
}

func init() {
	user := ptrace.User{}
	userType := reflect.TypeOf(user)

	field, ok := userType.FieldByName("UDebugReg")
	if !ok {
		panic("should never happen")
	}
	userDebugRegistersOffset = field.Offset
}
