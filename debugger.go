package bad

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"syscall"

	"github.com/pattyshack/bad/ptrace"
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

type Debugger struct {
	tracer *ptrace.Tracer

	*RegisterSet

	BreakPointSites *StopPointSet

	Pid         int
	ownsProcess bool

	state ProcessState
}

func newDebugger(tracer *ptrace.Tracer, ownsProcess bool) (*Debugger, error) {
	db := &Debugger{
		tracer:          tracer,
		RegisterSet:     NewRegisterSet(),
		BreakPointSites: NewBreakPointSites(tracer),
		Pid:             tracer.Pid(),
		ownsProcess:     ownsProcess,
		state:           newRunningProcessState(tracer.Pid()),
	}

	_, err := db.waitForSignal()
	if err != nil {
		_ = tracer.Detach()
		return nil, err
	}

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

	err := db.tracer.Resume(0)
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

func (db *Debugger) waitForSignal() (ProcessState, error) {
	// NOTE: golang does not support waitpid
	var status syscall.WaitStatus
	_, err := syscall.Wait4(db.Pid, &status, 0, nil)
	if err != nil {
		return ProcessState{}, fmt.Errorf(
			"failed to wait for process %d: %w",
			db.Pid,
			err)
	}

	db.state = newProcessState(db.Pid, status)

	if db.state.Stopped() {
		nextInstructionAddr, err := db.getProgramCounter()
		if err != nil {
			return ProcessState{}, fmt.Errorf(
				"failed to wait for process %d: %w",
				db.Pid,
				err)
		}

		db.state.NextInstructionAddress = nextInstructionAddr

		if db.state.StopSignal() == syscall.SIGTRAP {
			// NOTE: currentInstructionAddr may not be a valid instruction address
			// since x64 instruction could span multiple bytes.  However, since break
			// point sites are implemented using the int3 (0xcc), we know for sure
			// the address is valid if the current instruction is a break point site.
			currentInstructionAddr := nextInstructionAddr - 1

			site, ok := db.BreakPointSites.Get(currentInstructionAddr)
			if ok && site.IsEnabled() {
				// set pc back to int3's address
				err := db.setProgramCounter(currentInstructionAddr)
				if err != nil {
					return ProcessState{}, fmt.Errorf(
						"failed to reset program counter at break point: %w",
						err)
				}
			}
		}
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

func (db *Debugger) getProgramCounter() (VirtualAddress, error) {
	state, err := db.GetRegisterState()
	if err != nil {
		return 0, fmt.Errorf("failed to read program counter: %w", err)
	}

	rip, ok := db.RegisterByName("rip")
	if !ok {
		panic("should never happen")
	}

	return VirtualAddress(state.Value(rip).ToUint64()), nil
}

func (db *Debugger) setProgramCounter(addr VirtualAddress) error {
	state, err := db.GetRegisterState()
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

	err = db.SetRegisterState(newState)
	if err != nil {
		return fmt.Errorf("failed to set program counter to %s: %w", addr, err)
	}

	db.state.NextInstructionAddress = addr
	return nil
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
