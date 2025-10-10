package debugger

import (
	"fmt"
	"syscall"

	"golang.org/x/arch/x86/x86asm"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/registers"
	"github.com/pattyshack/bad/debugger/stoppoint"
	"github.com/pattyshack/bad/ptrace"
)

type ThreadState struct {
	Tid          int
	threadTracer *ptrace.Tracer

	Registers *registers.Registers

	CallStack *CallStack

	status                   *ThreadStatus
	expectsSyscallExit       bool
	hasPendingSigStop        bool
	hasPendingSingleStepTrap bool // toggled within step instruction only

	*Debugger // access to shared process resources
}

func (thread *ThreadState) Status() *ThreadStatus {
	return thread.status
}

func (thread *ThreadState) updateStatus(
	waitStatus syscall.WaitStatus,
	newlyCreated bool,
) error {
	// NOTE: we will immediately update the thread.status with a simple status
	// since ProcessState.Close needs accurate state to clean up properly.
	// We'll supplement this with a detailed status whenever possible, which
	// provide debug information to the user.
	thread.status = newSimpleWaitingStatus(thread.Tid, waitStatus)

	status, shouldResetProgramCounter, err := newDetailedWaitingStatus(
		thread,
		waitStatus)
	if err != nil {
		return fmt.Errorf("failed to wait for thread %d: %w", thread.Tid, err)
	}

	if shouldResetProgramCounter {
		err := thread.Registers.SetProgramCounter(status.NextInstructionAddress)
		if err != nil {
			return fmt.Errorf(
				"failed to wait for thread %d. "+
					"cannot reset program counter at break point: %w",
				thread.Tid,
				err)
		}
	}

	if status.Stopped {
		if status.IsInternalSigStop {
			thread.hasPendingSigStop = false
		}

		if thread.shouldUpdateSharedLibraries(status) {
			err = thread.updateSharedLibraries()
			if err != nil {
				return fmt.Errorf("failed to update shared libs: %w", err)
			}
		}

		state, err := thread.Registers.GetState()
		if err != nil {
			return fmt.Errorf(
				"failed to update call stack for thread %d: %w",
				thread.Tid,
				err)
		}

		err = thread.CallStack.Update(status, state)
		if err != nil {
			return fmt.Errorf(
				"failed to update call stack for thread %d: %w",
				thread.Tid,
				err)
		}

		if status.StopSignal == syscall.SIGTRAP {
			if status.TrapKind == SyscallTrap {
				thread.expectsSyscallExit = !thread.expectsSyscallExit
			} else {
				// In case syscall catch point got disabled after syscall entry, but
				// before syscall exit.
				thread.expectsSyscallExit = false
			}
		}
	}

	if newlyCreated {
		if !status.Stopped {
			panic("should never happen")
		}

		if status.StopSignal == syscall.SIGSTOP {
			status.IsInternalSigStop = true
		} else if status.StopSignal != syscall.SIGTRAP || thread.Tid != thread.Pid {
			panic("should never happen")
		}
	}

	thread.hasPendingSingleStepTrap = false
	thread.status = status

	return nil
}

func (thread *ThreadState) stepInstruction(
	bypassEnabledSitesAtCurrentPC bool,
	stepOverCall bool,
) error {
	var enabledSites stoppoint.StopSites
	if bypassEnabledSitesAtCurrentPC {
		enabledSites = thread.stopSites.GetEnabledAt(
			thread.status.NextInstructionAddress)
	}
	err := enabledSites.Disable()
	if err != nil {
		return fmt.Errorf(
			"failed to step instruction for thread %d: %w",
			thread.Tid,
			err)
	}

	var stepOverAddress *VirtualAddress
	if stepOverCall {
		instructions, err := thread.Disassemble(
			thread.status.NextInstructionAddress,
			1)
		if err != nil {
			return fmt.Errorf("failed to determine instruction type: %w", err)
		}

		// NOTE: golang's x86asm package is unable to disassemble all x64
		// instructions. When that happens, we'll simply assume the instruction is
		// not a call instruction.
		if len(instructions) == 1 {
			inst := instructions[0]
			if inst.Op == x86asm.CALL {
				addr := thread.status.NextInstructionAddress + VirtualAddress(inst.Len)
				stepOverAddress = &addr
			}
		}
	}

	err = thread.threadTracer.SingleStep()
	if err != nil {
		return fmt.Errorf(
			"failed to single step for thread %d: %w",
			thread.Tid,
			err)
	}

	thread.hasPendingSingleStepTrap = true
	_, err = thread.waitForSignalFromAnyThread()
	if err != nil {
		return fmt.Errorf(
			"failed to wait for step instruction for thread %d: %w",
			thread.Tid,
			err)
	}

	err = enabledSites.Enable()
	if err != nil {
		return fmt.Errorf(
			"failed to step instruction for thread %d: %w",
			thread.Tid,
			err)
	}

	if stepOverAddress == nil ||
		*stepOverAddress == thread.status.NextInstructionAddress {

		return nil
	}

	return thread.resumeUntilAddressOrSignal(*stepOverAddress)
}

func (thread *ThreadState) maybeSwallowInternalSigStop() error {
	if !thread.hasPendingSigStop {
		return nil
	}

	originalPC := thread.status.NextInstructionAddress

	// In theory, multiple signals could be queued up.  We'll keep resuming until
	// we hit a sig stop.
	for thread.status.Stopped {
		err := thread.threadTracer.Resume(0)
		if err != nil {
			return fmt.Errorf("failed to resume thread %d: %w", thread.Tid, err)
		}

		waitStatus, err := thread.signal.FromThread(thread.Tid)
		if err != nil {
			return fmt.Errorf("failed to wait for thread %d: %w", thread.Tid, err)
		}

		err = thread.updateStatus(waitStatus, false)
		if err != nil {
			return fmt.Errorf(
				"failed to update status for thread %d: %w",
				thread.Tid,
				err)
		}

		if thread.status.Stopped &&
			thread.status.NextInstructionAddress != originalPC {

			panic("thread advanced pc unexpectedly")
		}

		if thread.status.IsInternalSigStop {
			break
		}
	}

	return nil
}

func (thread *ThreadState) maybeBypassCurrentPCBreakSite() error {
	err := thread.maybeSwallowInternalSigStop()
	if err != nil {
		return err
	}

	enabledSites := thread.stopSites.GetEnabledAt(
		thread.status.NextInstructionAddress)
	if len(enabledSites) > 0 {
		err = thread.stepInstruction(true, false)
		if err != nil {
			return fmt.Errorf("failed to resume thread %d: %w", thread.Tid, err)
		}
	}

	return nil
}

func (thread *ThreadState) resume() error {
	var err error
	if thread.SyscallCatchPolicy.IsEnabled() {
		err = thread.threadTracer.SyscallTrappedResume(0)
	} else {
		err = thread.threadTracer.Resume(0)
	}

	if err != nil {
		return fmt.Errorf("failed to resume thread %d: %w", thread.Tid, err)
	}

	thread.status = newRunningStatus(thread.Tid)
	return nil
}

func (thread *ThreadState) resumeUntilAddressOrSignal(
	address VirtualAddress,
) error {
	site, err := thread.stopSites.Allocate(
		address,
		stoppoint.NewBreakSiteType(false))
	if err != nil {
		return fmt.Errorf("failed to allocate internal break site: %w", err)
	}

	isInternalOnly := !site.IsEnabled()
	if isInternalOnly {
		err = site.Enable()
		if err != nil {
			return fmt.Errorf("failed to enable internal break site: %w", err)
		}
	}

	_, err = thread.resumeUntilSignal(thread)
	if err != nil {
		return fmt.Errorf("failed to resume until address %s: %w", address, err)
	}

	if isInternalOnly {
		if thread.status.Stopped &&
			thread.status.StopSignal == syscall.SIGTRAP &&
			thread.status.TrapKind == SoftwareTrap &&
			thread.status.NextInstructionAddress == address {

			// Covert status to single step since the internal break site is
			// the only site enabled at the address. Note that we must clear
			// matched stop points since there could be user defined stop points
			// at the address, all of which are disabled.
			thread.status.TrapKind = SingleStepTrap
			thread.status.StopPoints = nil
		}

		err = site.Disable()
		if err != nil {
			return fmt.Errorf("failed to disable internal break site: %w", err)
		}
	}

	err = site.Deallocate()
	if err != nil {
		return fmt.Errorf("failed to deallocate internal break site: %w", err)
	}

	return nil
}

func (thread *ThreadState) maybeStepOverFunctionPrologue() error {
	pc := thread.status.NextInstructionAddress
	_, funcEntry, err := thread.LoadedElves.FunctionEntryContainingAddress(pc)
	if err != nil {
		return err
	} else if funcEntry == nil {
		return nil
	}

	ars, err := funcEntry.AddressRanges()
	if err != nil {
		return err
	} else if len(ars) == 0 {
		return nil
	}

	// If the pc is in a function's prologue, advance the pc to the body
	prologueAddr, err := thread.LoadedElves.ToVirtualAddress(
		funcEntry.File.File,
		ars[0].Low)
	if err != nil {
		return err
	}

	prologue, err := thread.LoadedElves.LineEntryAt(prologueAddr)
	if err != nil {
		return err
	} else if prologue == nil {
		return nil
	}

	body, err := prologue.Next()
	if err != nil {
		return err
	} else if body == nil {
		return fmt.Errorf("body line entry not found")
	}

	bodyAddr, err := thread.LoadedElves.LineEntryToVirtualAddress(body)
	if err != nil {
		return err
	}

	if prologueAddr <= pc && pc < bodyAddr {
		err := thread.resumeUntilAddressOrSignal(bodyAddr)
		return err
	}

	return nil
}

func (thread *ThreadState) stepUntilDifferentLine(stepOver bool) error {
	origLine, err := thread.LoadedElves.LineEntryAt(
		thread.status.NextInstructionAddress)
	if err != nil {
		return err
	}

	mustAdvance := true
	for {
		codeRanges := thread.CallStack.UnexecutedInlinedFunctionCodeRanges()
		var endAddress *VirtualAddress
		if stepOver && len(codeRanges) > 0 {
			high := codeRanges[len(codeRanges)-1].High
			endAddress = &high
		}

		if mustAdvance || endAddress == nil {
			err := thread.stepInstruction(mustAdvance, stepOver)
			if err != nil {
				return err
			}
		}
		mustAdvance = false

		if endAddress != nil &&
			*endAddress != thread.status.NextInstructionAddress {

			err := thread.resumeUntilAddressOrSignal(*endAddress)
			if err != nil {
				return err
			}
		}

		if thread.status.TrapKind != SingleStepTrap {
			return nil
		}

		line, err := thread.LoadedElves.LineEntryAt(
			thread.status.NextInstructionAddress)
		if err != nil {
			return err
		}

		if line == nil {
			return nil
		} else if line.EndSequence {
			continue
		} else if origLine.FileEntry.Name == line.FileEntry.Name &&
			origLine.Line == line.Line {

			continue
		} else {
			return nil
		}
	}
}
