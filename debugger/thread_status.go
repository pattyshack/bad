package debugger

import (
	"bytes"
	"fmt"
	"path"
	"syscall"

	"github.com/pattyshack/bad/debugger/catchpoint"
	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/stoppoint"
	"github.com/pattyshack/bad/dwarf"
	"github.com/pattyshack/bad/elf"
	"github.com/pattyshack/bad/ptrace"
)

const (
	syscallTrapSignal = syscall.SIGTRAP | 0x80

	// NOTE: clone ptrace event use bits aren't part of the stop signal.
	// The event is triggered on the clone caller thread.  A corresponding
	// sig stop is trigger by the newly thread.
	cloneTrapExtendedSignal = int(syscall.SIGTRAP) | int(ptrace.EVENT_CLONE<<8)
)

type ThreadStatus struct {
	Tid int

	Stopped    bool
	StopSignal syscall.Signal

	// Only populated when thread is stopped by SIGSTOP
	IsInternalSigStop bool

	Signaled bool
	Signal   syscall.Signal

	Exited     bool
	ExitStatus int

	// Only populated when thread is stopped.
	NextInstructionAddress VirtualAddress

	// Only populated when thread is stopped
	FunctionName string

	// Only populated when thread is stopped (populated as part of call stack
	// update)
	*dwarf.FileEntry
	Line int64

	// Only populated when thread is stopped by SIGTRAP
	TrapKind

	// Only populated when thread is stopped by break points / watch points
	StopPoints []stoppoint.Triggered

	// Only populated when thread is stopped by SyscallTrap
	SyscallTrapInfo *catchpoint.SyscallTrapInfo
}

func (status ThreadStatus) Running() bool {
	return !status.Stopped && !status.Signaled && !status.Exited
}

func (status ThreadStatus) String() string {
	if status.Running() {
		return fmt.Sprintf("thread %d running", status.Tid)
	} else if status.Stopped {
		reason := ""
		if status.StopSignal == syscall.SIGTRAP &&
			status.TrapKind != UnknownTrap {

			reason = fmt.Sprintf(" (%s)", status.TrapKind)

			for _, triggered := range status.StopPoints {
				point := triggered.StopPoint
				site := triggered.StopSite

				dataStr := ""
				if point.Type().IsWatchPoint {
					dataStr = " (data:"
					for _, b := range site.Data() {
						dataStr += fmt.Sprintf(" 0x%02x", b)
					}

					if !bytes.Equal(site.PreviousData(), site.Data()) {
						dataStr += " ; previous:"
						for _, b := range site.PreviousData() {
							dataStr += fmt.Sprintf(" 0x%02x", b)
						}
					}

					dataStr += ")"
				}

				reason += fmt.Sprintf("\n    %s (id=%d)", point.Type(), point.Id())
				reason += fmt.Sprintf("\n      resolver: %s", point.Resolver())
				reason += fmt.Sprintf("\n      triggered: %s%s", site.Key(), dataStr)
			}

			if status.SyscallTrapInfo != nil {
				reason += "\n" + status.SyscallTrapInfo.String()
			}
		}

		onLine := ""
		if status.FileEntry != nil {
			onLine = fmt.Sprintf(" %s:%d", status.FileEntry.Path(), status.Line)
		}

		inFunc := ""
		if status.FunctionName != "" {
			inFunc = " (" + status.FunctionName + ")"
		}

		return fmt.Sprintf(
			"thread %d stopped\n  at: %s%s%s\n  with signal: %v%s",
			status.Tid,
			status.NextInstructionAddress,
			onLine,
			inFunc,
			status.StopSignal,
			reason)
	} else if status.Signaled {
		return fmt.Sprintf(
			"thread %d terminated with signal: %v",
			status.Tid,
			status.Signal)
	} else if status.Exited {
		return fmt.Sprintf(
			"thread %d exited with status: %d",
			status.Tid,
			status.ExitStatus)
	} else {
		panic("shold never happen")
	}
}

func newRunningStatus(tid int) *ThreadStatus {
	return &ThreadStatus{
		Tid: tid,
	}
}

func newInlinedStepInStatus(
	status *ThreadStatus,
) *ThreadStatus {
	return &ThreadStatus{
		Tid:                    status.Tid,
		Stopped:                true,
		StopSignal:             syscall.SIGTRAP,
		NextInstructionAddress: status.NextInstructionAddress,
		FunctionName:           status.FunctionName,
		TrapKind:               SingleStepTrap,
	}
}

func newSimpleWaitingStatus(
	tid int,
	waitStatus syscall.WaitStatus,
) *ThreadStatus {
	return &ThreadStatus{
		Tid:        tid,
		Stopped:    waitStatus.Stopped(),
		StopSignal: waitStatus.StopSignal(),
		Signaled:   waitStatus.Signaled(),
		Signal:     waitStatus.Signal(),
		Exited:     waitStatus.Exited(),
		ExitStatus: waitStatus.ExitStatus(),
	}
}

// Note: this creates a new ThreadStatus without making any modification to
// the debugger's internal state.
func newDetailedWaitingStatus(
	thread *ThreadState,
	waitStatus syscall.WaitStatus,
) (
	*ThreadStatus,
	bool, // should reset program counter
	error,
) {
	status := newSimpleWaitingStatus(thread.Tid, waitStatus)

	if !status.Stopped {
		return status, false, nil
	}

	registerState, pc, err := thread.Registers.GetProgramCounter()
	if err != nil {
		return nil, false, err
	}

	if status.StopSignal == syscall.SIGSTOP {
		status.IsInternalSigStop = thread.hasPendingSigStop
	}

	shouldResetProgramCounter := false
	if status.StopSignal == syscallTrapSignal {
		// Replaced the modified syscall trap signal with a normal trap signal.
		status.StopSignal = syscall.SIGTRAP
		status.TrapKind = SyscallTrap

		if thread.expectsSyscallExit { // syscall returned
			status.SyscallTrapInfo = catchpoint.NewSyscallTrapExitInfo(registerState)
		} else { // syscall entry
			status.SyscallTrapInfo = catchpoint.NewSyscallTrapEntryInfo(registerState)
		}
	} else if status.StopSignal == syscall.SIGTRAP {
		// NOTE: clone ptrace event use bits aren't part of the stop signal.
		if int(waitStatus>>8) == cloneTrapExtendedSignal {
			status.TrapKind = CloneTrap
		} else {
			sigInfo, err := thread.threadTracer.GetSigInfo()
			if err != nil {
				return nil, false, err
			}

			status.TrapKind = TrapCodeToKind(sigInfo.Code)
		}

		realPC, siteKeys, err := thread.stopSites.ListTriggered(
			pc,
			status.TrapKind)
		if err != nil {
			return nil, false, err
		}

		shouldResetProgramCounter = pc != realPC
		pc = realPC

		triggered := thread.BreakPoints.Match(siteKeys)
		triggered = append(triggered, thread.WatchPoints.Match(siteKeys)...)
		status.StopPoints = triggered

		if status.TrapKind == SoftwareTrap && len(status.StopPoints) == 0 {
			_, ok := thread.rendezvousAddresses[pc]
			if ok {
				status.TrapKind = RendezvousTrap
			}
		}
	}

	status.NextInstructionAddress = pc

	funcEntry, err := thread.LoadedElves.FunctionEntryContainingAddress(
		pc)
	if err != nil {
		return nil, false, err
	}

	if funcEntry != nil {
		name, _, err := funcEntry.Name()
		if err != nil {
			return nil, false, err
		}

		prefix := ""
		if funcEntry.CompileUnit.FileName != "" {
			prefix = path.Base(funcEntry.CompileUnit.FileName) + "|"
		}
		status.FunctionName = prefix + name
	}

	if status.FunctionName == "" {
		symbol := thread.LoadedElves.SymbolSpans(pc)
		if symbol != nil && symbol.Type() == elf.SymbolTypeFunction {
			prefix := ""
			if symbol.Parent.File().FileName != "" {
				prefix = path.Base(symbol.Parent.File().FileName) + "|"
			}
			status.FunctionName = prefix + symbol.PrettyName()
		}
	}

	return status, shouldResetProgramCounter, nil
}
