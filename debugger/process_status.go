package debugger

import (
	"bytes"
	"fmt"
	"syscall"

	"github.com/pattyshack/bad/debugger/catchpoint"
	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/stoppoint"
	"github.com/pattyshack/bad/dwarf"
	"github.com/pattyshack/bad/elf"
)

type ProcessStatus struct {
	Pid int

	Stopped    bool
	StopSignal syscall.Signal

	Signaled bool
	Signal   syscall.Signal

	Exited     bool
	ExitStatus int

	// Only populated when process is stopped.
	NextInstructionAddress VirtualAddress

	// Only populated when process is stopped
	FunctionName string

	// Only populated when process is stopped (populated as part of call stack
	// update)
	*dwarf.FileEntry
	Line int64

	// Only populated when process is stopped by SIGTRAP
	TrapKind

	// Only populated when process is stopped by break points / watch points
	StopPoints []stoppoint.Triggered

	// Only populated when process is stopped by SyscallTrap
	SyscallTrapInfo *catchpoint.SyscallTrapInfo
}

func (status ProcessStatus) Running() bool {
	return !status.Stopped && !status.Signaled && !status.Exited
}

func (status ProcessStatus) String() string {
	if status.Running() {
		return fmt.Sprintf("process %d running", status.Pid)
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
			"process %d stopped\n  at: %s%s%s\n  with signal: %v%s",
			status.Pid,
			status.NextInstructionAddress,
			onLine,
			inFunc,
			status.StopSignal,
			reason)
	} else if status.Signaled {
		return fmt.Sprintf(
			"process %d terminated with signal: %v",
			status.Pid,
			status.Signal)
	} else if status.Exited {
		return fmt.Sprintf(
			"process %d exited with status: %d",
			status.Pid,
			status.ExitStatus)
	} else {
		panic("shold never happen")
	}
}

func newInlinedStepInStatus(
	status *ProcessStatus,
) *ProcessStatus {
	return &ProcessStatus{
		Pid:                    status.Pid,
		Stopped:                true,
		StopSignal:             syscall.SIGTRAP,
		NextInstructionAddress: status.NextInstructionAddress,
		FunctionName:           status.FunctionName,
		TrapKind:               SingleStepTrap,
	}
}

// Note: this creates a new ProcessStatus without making any modification to
// the debugger's internal state.
func newWaitingStatus(
	proc *Debugger,
	waitStatus syscall.WaitStatus,
) (
	*ProcessStatus,
	bool, // should reset program counter
	error,
) {
	status := &ProcessStatus{
		Pid:        proc.Pid,
		Stopped:    waitStatus.Stopped(),
		StopSignal: waitStatus.StopSignal(),
		Signaled:   waitStatus.Signaled(),
		Signal:     waitStatus.Signal(),
		Exited:     waitStatus.Exited(),
		ExitStatus: waitStatus.ExitStatus(),
	}

	if !status.Stopped {
		return status, false, nil
	}

	registerState, pc, err := proc.Registers.GetProgramCounter()
	if err != nil {
		return nil, false, err
	}

	shouldResetProgramCounter := false
	if status.StopSignal == syscallTrapSignal {
		// Replaced the modified syscall trap signal with a normal trap signal.
		status.StopSignal = syscall.SIGTRAP
		status.TrapKind = SyscallTrap

		if proc.expectsSyscallExit { // syscall returned
			status.SyscallTrapInfo = catchpoint.NewSyscallTrapExitInfo(registerState)
		} else { // syscall entry
			status.SyscallTrapInfo = catchpoint.NewSyscallTrapEntryInfo(registerState)
		}
	} else if status.StopSignal == syscall.SIGTRAP {
		sigInfo, err := proc.tracer.GetSigInfo()
		if err != nil {
			return nil, false, err
		}

		status.TrapKind = TrapCodeToKind(sigInfo.Code)

		realPC, siteKeys, err := proc.stopSites.ListTriggered(pc, status.TrapKind)
		if err != nil {
			return nil, false, err
		}

		shouldResetProgramCounter = pc != realPC
		pc = realPC

		triggered := proc.BreakPoints.Match(siteKeys)
		triggered = append(triggered, proc.WatchPoints.Match(siteKeys)...)
		status.StopPoints = triggered
	}

	status.NextInstructionAddress = pc

	funcEntry, err := proc.LoadedElf.FunctionEntryContainingAddress(pc)
	if err != nil {
		return nil, false, err
	}

	if funcEntry != nil {
		name, _, err := funcEntry.Name()
		if err != nil {
			return nil, false, err
		}
		status.FunctionName = name
	}

	if status.FunctionName == "" {
		symbol := proc.LoadedElf.SymbolSpans(pc)
		if symbol != nil && symbol.Type() == elf.SymbolTypeFunction {
			status.FunctionName = symbol.PrettyName()
		}
	}

	return status, shouldResetProgramCounter, nil
}
