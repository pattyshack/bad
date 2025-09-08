package bad

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"syscall"

	"github.com/pattyshack/bad/ptrace"
)

var (
	userDebugRegistersOffset = uintptr(0) // initialized by init()
)

type VirtualAddress uint64

func (addr VirtualAddress) String() string {
	return fmt.Sprintf("0x%x", uint64(addr))
}

type ProcessState string

const (
	Stopped    = ProcessState("stopped")
	Running    = ProcessState("running")
	Exited     = ProcessState("exited")
	Terminated = ProcessState("terminated")
)

type Debugger struct {
	tracer *ptrace.Tracer

	*RegisterSet

	Pid         int
	ownsProcess bool

	processState ProcessState
}

func AttachTo(pid int) (*Debugger, error) {
	tracer, err := ptrace.AttachToProcess(pid)
	if err != nil {
		return nil, err
	}

	db := &Debugger{
		tracer:       tracer,
		RegisterSet:  NewRegisterSet(),
		Pid:          tracer.Pid(),
		ownsProcess:  false,
		processState: Stopped,
	}

	_, err = db.WaitForSignal()
	if err != nil {
		_ = tracer.Detach()
		return nil, err
	}

	return db, nil
}

func StartAndAttachTo(cmd *exec.Cmd) (*Debugger, error) {
	tracer, err := ptrace.StartAndAttachToProcess(cmd)
	if err != nil {
		return nil, err
	}

	db := &Debugger{
		tracer:       tracer,
		RegisterSet:  NewRegisterSet(),
		Pid:          tracer.Pid(),
		ownsProcess:  true,
		processState: Stopped,
	}

	_, err = db.WaitForSignal()
	if err != nil {
		_ = tracer.Detach()
		return nil, err
	}

	return db, nil
}

func StartCmdAndAttachTo(name string, args ...string) (*Debugger, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return StartAndAttachTo(cmd)
}

func (db *Debugger) Resume() error {
	err := db.tracer.Resume(0)
	if err != nil {
		return err
	}

	db.processState = Running

	fmt.Println("process", db.Pid, "running")
	return nil
}

func (db *Debugger) WaitForSignal() (syscall.WaitStatus, error) {
	// NOTE: golang does not support waitpid
	var status syscall.WaitStatus
	_, err := syscall.Wait4(db.Pid, &status, 0, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to wait for process %d: %w", db.Pid, err)
	}

	if status.Exited() {
		db.processState = Exited
	} else if status.Signaled() {
		db.processState = Terminated
	} else if status.Stopped() {
		db.processState = Stopped
	} else {
		panic(fmt.Sprintf("Unhandled wait status: %v", status))
	}

	if status.Exited() {
		fmt.Printf(
			"process %d exited with status: %d\n",
			db.Pid,
			status.ExitStatus())
	} else if status.Signaled() {
		fmt.Printf(
			"process %d terminated with signal: %v\n",
			db.Pid,
			status.Signal())
	} else if status.Stopped() {
		pc, err := db.GetProgramCounter()
		if err != nil {
			return 0, err
		}

		fmt.Printf(
			"process %d stopped at %s with signal: %v\n",
			db.Pid,
			pc,
			status.StopSignal())
	}

	return status, nil
}

func (db *Debugger) Signal(signal syscall.Signal) error {
	err := syscall.Kill(db.Pid, signal)
	if err != nil {
		return fmt.Errorf("failed to signal to process %d (%v): %w",
			db.Pid,
			signal,
			err)
	}

	fmt.Printf("signaled to process %d: %s\n", db.Pid, signal)
	return nil
}

func (db *Debugger) Close() error {
	if db.processState == Running {
		err := db.Signal(syscall.SIGSTOP)
		if err != nil {
			return err
		}

		_, err = db.WaitForSignal()
		if err != nil {
			return err
		}
	}

	err := db.tracer.Detach()
	if err != nil {
		return err
	}

	err = db.Signal(syscall.SIGCONT)
	if err != nil {
		return err
	}

	if db.ownsProcess {
		err = db.Signal(syscall.SIGKILL)
		if err != nil {
			return err
		}

		_, err = db.WaitForSignal()
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *Debugger) GetRegisterState() (RegisterState, error) {
	if db.processState != Stopped {
		return RegisterState{}, fmt.Errorf(
			"cannot get register state. process %d %s (not stopped)",
			db.Pid,
			db.processState)
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
	if db.processState != Stopped {
		return fmt.Errorf(
			"cannot set register state. process %d %s (not stopped)",
			db.Pid,
			db.processState)
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

func (db *Debugger) GetProgramCounter() (VirtualAddress, error) {
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

func init() {
	user := ptrace.User{}
	userType := reflect.TypeOf(user)

	field, ok := userType.FieldByName("UDebugReg")
	if !ok {
		panic("should never happen")
	}
	userDebugRegistersOffset = field.Offset
}
