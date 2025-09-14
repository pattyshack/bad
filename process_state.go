package bad

import (
	"bytes"
	"fmt"
	"syscall"
)

type TrapReason string

const (
	UnknownTrap    = TrapReason("")
	SoftwareTrap   = TrapReason("software break")
	HardwareTrap   = TrapReason("hardware break")
	SingleStepTrap = TrapReason("single step")
	SyscallTrap    = TrapReason("syscall trap")
)

func TrapCodeToReason(code int32) TrapReason {
	// NOTE: on x64, linux incorrect report software trap as SI_KERNEL (0x80)
	// when it should have reported of TRAP_BRKPT (1).
	switch code {
	case 0x80: // SI_KERNEL
		return SoftwareTrap
	case 4: // TRAP_HWBKPT
		return HardwareTrap
	case 2: // TRAP_TRACE
		return SingleStepTrap
	default:
		// Most si_code values are not handled.  e.g, SI_TKILL (-6)
		return UnknownTrap
	}
}

type SyscallTrapInfo struct {
	IsEntry bool

	Id   SyscallId
	Args [6]uint64
	Ret  uint64
}

func (info SyscallTrapInfo) String() string {
	result := "syscall " + info.Id.Name
	if info.IsEntry {
		result += " entry:"
		for _, arg := range info.Args {
			result += fmt.Sprintf(" 0x%x", arg)
		}
	} else {
		result += fmt.Sprintf(" returned: 0x%x", info.Ret)
	}

	return result
}

type ProcessState struct {
	Pid int

	Status *syscall.WaitStatus // nil indicates running

	// Only populated when process is stopped.
	NextInstructionAddress VirtualAddress

	// Only populated when process is stopped by SIGTRAP
	TrapReason

	// Only populated when process is stopped by break points / watch points
	StopPoints []StopPoint

	// Only populated when process is stopped by SyscallTrap
	SyscallTrapInfo *SyscallTrapInfo
}

func newRunningProcessState(pid int) ProcessState {
	return ProcessState{
		Pid: pid,
	}
}

func (state ProcessState) Running() bool {
	return state.Status == nil
}

func (state ProcessState) Stopped() bool {
	return state.Status != nil && state.Status.Stopped()
}

func (state ProcessState) StopSignal() syscall.Signal {
	if state.Status == nil {
		return -1
	}

	return state.Status.StopSignal()
}

func (state ProcessState) Signaled() bool {
	return state.Status != nil && state.Status.Signaled()
}

func (state ProcessState) Signal() syscall.Signal {
	if state.Status == nil {
		return -1
	}
	return state.Status.Signal()
}

func (state ProcessState) Exited() bool {
	return state.Status != nil && state.Status.Exited()
}

func (state ProcessState) ExitStatus() int {
	if state.Status == nil {
		return -1
	}

	return state.Status.ExitStatus()
}

func (state ProcessState) String() string {
	if state.Running() {
		return fmt.Sprintf("process %d running", state.Pid)
	} else if state.Stopped() {
		reason := ""
		if state.StopSignal() == syscall.SIGTRAP &&
			state.TrapReason != UnknownTrap {

			reason = fmt.Sprintf(" (%s)", state.TrapReason)

			for _, sp := range state.StopPoints {
				dataStr := ""
				if !sp.Type().IsBreakPoint {
					dataStr = ". (data:"
					for _, b := range sp.Data() {
						dataStr += fmt.Sprintf(" 0x%02x", b)
					}

					if !bytes.Equal(sp.PreviousData(), sp.Data()) {
						dataStr += " ; previous:"
						for _, b := range sp.PreviousData() {
							dataStr += fmt.Sprintf(" 0x%02x", b)
						}
					}

					dataStr += ")"
				}
				reason += fmt.Sprintf("\n%s at %s%s", sp.Type(), sp.Address(), dataStr)
			}

			if state.SyscallTrapInfo != nil {
				reason += "\n" + state.SyscallTrapInfo.String()
			}
		}

		return fmt.Sprintf(
			"process %d stopped at %s with signal: %v%s",
			state.Pid,
			state.NextInstructionAddress,
			state.StopSignal(),
			reason)
	} else if state.Signaled() {
		return fmt.Sprintf(
			"process %d terminated with signal: %v",
			state.Pid,
			state.Signal())
	} else if state.Exited() {
		return fmt.Sprintf(
			"process %d exited with status: %d",
			state.Pid,
			state.ExitStatus())
	} else {
		panic("shold never happen")
	}
}
