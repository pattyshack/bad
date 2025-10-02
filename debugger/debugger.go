package debugger

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/arch/x86/x86asm"

	"github.com/pattyshack/bad/debugger/catchpoint"
	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/loadedelves"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/debugger/registers"
	"github.com/pattyshack/bad/debugger/stoppoint"
	"github.com/pattyshack/bad/elf"
	"github.com/pattyshack/bad/ptrace"
)

const (
	syscallTrapSignal = syscall.SIGTRAP | 0x80
)

type Debugger struct {
	tracer *ptrace.Tracer

	signal *Signaler

	VirtualMemory *memory.VirtualMemory
	Registers     *registers.Registers

	stopSites stoppoint.StopSitePool

	stoppoint.StopSiteResolverFactory

	BreakPoints *stoppoint.StopPointSet
	WatchPoints *stoppoint.StopPointSet

	SyscallCatchPolicy *catchpoint.SyscallCatchPolicy

	*memory.Disassembler

	CallStack *CallStack

	LoadedElves *loadedelves.Files
	*SourceFiles

	Pid         int
	ownsProcess bool

	status             *ProcessStatus
	expectsSyscallExit bool

	entryPointSite       stoppoint.StopSite
	rendezvousNotifySite stoppoint.StopSite
	rendezvousAddresses  map[VirtualAddress]struct{}
}

func newDebugger(tracer *ptrace.Tracer, ownsProcess bool) (*Debugger, error) {
	regs := registers.New(tracer)
	mem := memory.New(tracer)

	loadedElves := loadedelves.NewFiles(mem)

	stopSites := stoppoint.NewStopSitePool(regs, mem)

	db := &Debugger{
		tracer:                  tracer,
		signal:                  NewSignaler(tracer.Pid()),
		VirtualMemory:           mem,
		Registers:               regs,
		stopSites:               stopSites,
		StopSiteResolverFactory: stoppoint.NewStopSiteResolverFactory(loadedElves),
		BreakPoints:             stoppoint.NewBreakPointSet(stopSites),
		WatchPoints:             stoppoint.NewWatchPointSet(stopSites),
		SyscallCatchPolicy:      catchpoint.NewSyscallCatchPolicy(),
		Disassembler:            memory.NewDisassembler(mem, stopSites),
		CallStack:               newCallStack(loadedElves, mem),
		LoadedElves:             loadedElves,
		SourceFiles:             NewSourceFiles(),
		Pid:                     tracer.Pid(),
		ownsProcess:             ownsProcess,
		status:                  newRunningStatus(tracer.Pid()),
		rendezvousAddresses:     map[VirtualAddress]struct{}{},
	}

	db.signal.ForwardInterruptToProcess()

	waitStatus, err := db.signal.FromProcess()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	// NOTE: This simple status ensures we can Close properly.  We'll supplement
	// the status with additional detail below.
	db.status = newSimpleWaitingStatus(db.Pid, waitStatus)

	// NOTE: LoadBinary must be called after wait to avoid procfs data race
	// (the debugger could read procfs before the process entry point is
	// written to procfs)
	_, err = db.LoadedElves.LoadBinary(db.Pid)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	entryPointSite, err := db.stopSites.Allocate(
		db.LoadedElves.EntryPoint(),
		stoppoint.NewBreakSiteType(false))
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	err = entryPointSite.Enable()
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	db.entryPointSite = entryPointSite
	db.rendezvousAddresses[db.LoadedElves.EntryPoint()] = struct{}{}

	if ownsProcess {
		err = tracer.SetOptions(ptrace.O_EXITKILL | ptrace.O_TRACESYSGOOD)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf(
				"failed to set PTRACE_O_EXITKILL on process %d",
				tracer.Pid())
		}
	}

	// NOTE: when attaching to an existing process, updateStatus should be called
	// after LoadBinary since the status extract data from the loaded elf file.
	//
	// When starting a new child process, it doesn't matter either way since
	// the child is interrupted before the program entry point.
	err = db.updateStatus(waitStatus)
	if err != nil {
		_ = db.Close()
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

func (db *Debugger) Status() *ProcessStatus {
	return db.status
}

func (db *Debugger) ResumeUntilSignal() (*ProcessStatus, error) {
	if db.status.Exited {
		return nil, fmt.Errorf(
			"failed to resume process %d: %w",
			db.Pid,
			ErrProcessExited)
	}

	enabledSites := db.stopSites.GetEnabledAt(db.status.NextInstructionAddress)
	if len(enabledSites) > 0 {
		_, err := db.StepInstruction()
		if err != nil {
			return nil, fmt.Errorf("failed to resume process %d: %w", db.Pid, err)
		}
	}

	err := db.resumeUntilSignal()
	if err != nil {
		return nil, err
	}

	return db.status, nil
}

func (db *Debugger) StepInstruction() (*ProcessStatus, error) {
	if db.status.Exited {
		return nil, fmt.Errorf(
			"failed to step instruction for process %d: %w",
			db.Pid,
			ErrProcessExited)
	}

	enabledSites := db.stopSites.GetEnabledAt(db.status.NextInstructionAddress)
	err := enabledSites.Disable()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to step instruction for process %d: %w",
			db.Pid,
			err)
	}

	err = db.stepInstruction(false)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to step instruction for process %d: %w",
			db.Pid,
			err)
	}

	err = enabledSites.Enable()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to step instruction for process %d: %w",
			db.Pid,
			err)
	}

	return db.status, nil
}

func (db *Debugger) StepIn() (*ProcessStatus, error) {
	if db.status.Exited {
		return nil, fmt.Errorf(
			"failed to step in for process %d: %w",
			db.Pid,
			ErrProcessExited)
	}

	inlinedStepInStatus, err := db.CallStack.MaybeStepIntoInlinedFunction(
		db.status)
	if err != nil {
		return nil, fmt.Errorf("failed to step in for process %d: %w", db.Pid, err)
	}

	if inlinedStepInStatus != nil {
		db.status = inlinedStepInStatus
		db.expectsSyscallExit = false
		return db.status, nil
	}

	enabledSites := db.stopSites.GetEnabledAt(db.status.NextInstructionAddress)
	err = enabledSites.Disable()
	if err != nil {
		return nil, fmt.Errorf("failed to step in for process %d: %w", db.Pid, err)
	}

	err = db.stepUntilDifferentLine(false)
	if err != nil {
		return nil, err
	}

	err = db.maybeStepOverFunctionPrologue()
	if err != nil {
		return nil, err
	}

	err = enabledSites.Enable()
	if err != nil {
		return nil, fmt.Errorf("failed to step in for process %d: %w", db.Pid, err)
	}

	return db.status, nil
}

func (db *Debugger) StepOver() (*ProcessStatus, error) {
	if db.status.Exited {
		return nil, fmt.Errorf(
			"failed to step over for process %d: %w",
			db.Pid,
			ErrProcessExited)
	}

	enabledSites := db.stopSites.GetEnabledAt(db.status.NextInstructionAddress)
	err := enabledSites.Disable()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to step over for process %d: %w",
			db.Pid,
			err)
	}

	err = db.stepUntilDifferentLine(true)
	if err != nil {
		return nil, err
	}

	err = enabledSites.Enable()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to step over for process %d: %w",
			db.Pid,
			err)
	}

	return db.status, nil
}

func (db *Debugger) StepOut() (*ProcessStatus, error) {
	if db.status.Exited {
		return nil, fmt.Errorf(
			"failed to step out for process %d: %w",
			db.Pid,
			ErrProcessExited)
	}

	enabledSites := db.stopSites.GetEnabledAt(db.status.NextInstructionAddress)
	err := enabledSites.Disable()
	if err != nil {
		return nil, fmt.Errorf("failed to step out for process %d: %w", db.Pid, err)
	}

	var returnAddress VirtualAddress
	frame := db.CallStack.CurrentFrame()
	if frame != nil && frame.IsInlined() {
		// XXX: This is not completely correct since the inlined function may
		// jump to any address, but is good enough for our purpose.
		returnAddress = frame.CodeRanges[len(frame.CodeRanges)-1].High
	} else {
		state, err := db.Registers.GetState()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to step out for process %d: %w",
				db.Pid,
				err)
		}

		framePointer := VirtualAddress(
			state.Value(registers.FramePointer).ToUint64())

		addressBytes := make([]byte, 8)
		n, err := db.VirtualMemory.Read(framePointer+8, addressBytes)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to step out for process %d: %w",
				db.Pid,
				err)
		}
		if n != 8 {
			panic("should never happen")
		}

		n, err = binary.Decode(addressBytes, binary.LittleEndian, &returnAddress)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to step out for process %d: %w",
				db.Pid,
				err)
		}
		if n != 8 {
			panic("should never happen")
		}
	}

	err = db.resumeUntilAddressOrSignal(returnAddress)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to step out for process %d: %w",
			db.Pid,
			err)
	}

	err = enabledSites.Enable()
	if err != nil {
		return nil, fmt.Errorf("failed to step out for process %d: %w", db.Pid, err)
	}

	return db.status, nil
}

func (db *Debugger) stepUntilDifferentLine(stepOver bool) error {
	origLine, err := db.LoadedElves.LineEntryAt(db.status.NextInstructionAddress)
	if err != nil {
		return err
	}

	for {
		codeRanges := db.CallStack.UnexecutedInlinedFunctionCodeRanges()
		if stepOver && len(codeRanges) > 0 {
			err := db.resumeUntilAddressOrSignal(codeRanges[len(codeRanges)-1].High)
			if err != nil {
				return err
			}
		} else {
			err := db.stepInstruction(stepOver)
			if err != nil {
				return err
			}
		}

		if db.status.TrapKind != SingleStepTrap {
			return nil
		}

		line, err := db.LoadedElves.LineEntryAt(db.status.NextInstructionAddress)
		if err != nil {
			return err
		}

		if line == nil {
			return nil
		} else if origLine == line || line.EndSequence {
			continue
		} else {
			return nil
		}
	}
}

func (db *Debugger) maybeStepOverFunctionPrologue() error {
	pc := db.status.NextInstructionAddress
	funcEntry, err := db.LoadedElves.FunctionEntryContainingAddress(pc)
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
	prologueAddr, err := db.LoadedElves.ToVirtualAddress(
		funcEntry.File.File,
		ars[0].Low)
	if err != nil {
		return err
	}

	prologue, err := db.LoadedElves.LineEntryAt(prologueAddr)
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

	bodyAddr, err := db.LoadedElves.LineEntryToVirtualAddress(body)
	if err != nil {
		return err
	}

	if prologueAddr <= pc && pc < bodyAddr {
		err := db.resumeUntilAddressOrSignal(bodyAddr)
		return err
	}

	return nil
}

func (db *Debugger) resumeUntilSignal() error {
	resume := db.tracer.Resume
	if db.SyscallCatchPolicy.IsEnabled() {
		resume = db.tracer.SyscallTrappedResume
	}

	err := resume(0)
	if err != nil {
		return fmt.Errorf("failed to resume process %d: %w", db.Pid, err)
	}
	db.status = newRunningStatus(db.Pid)

	for {
		err := db.waitForSignal()
		if err != nil {
			return fmt.Errorf("failed to resume process %d: %w", db.Pid, err)
		}

		if !db.status.Stopped || db.status.StopSignal != syscall.SIGTRAP {
			return nil
		}

		switch db.status.TrapKind {
		case SyscallTrap:
			if db.SyscallCatchPolicy.Matches(db.status.SyscallTrapInfo.Id) {
				return nil
			}
		case RendezvousTrap:
			_, err := db.StepInstruction()
			if err != nil {
				return fmt.Errorf("failed to resume process %d: %w", db.Pid, err)
			}
		default:
			return nil
		}

		err = resume(0)
		if err != nil {
			return fmt.Errorf("failed to resume process %d: %w", db.Pid, err)
		}
		db.status = newRunningStatus(db.Pid)
	}
}

func (db *Debugger) resumeUntilAddressOrSignal(address VirtualAddress) error {
	site, err := db.stopSites.Allocate(address, stoppoint.NewBreakSiteType(false))
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

	err = db.resumeUntilSignal()
	if err != nil {
		return fmt.Errorf("failed to resume until address %s: %w", address, err)
	}

	if isInternalOnly {
		if db.status.Stopped &&
			db.status.StopSignal == syscall.SIGTRAP &&
			db.status.TrapKind == SoftwareTrap &&
			db.status.NextInstructionAddress == address {

			// Covert status to single step since the internal break site is
			// the only site enabled at the address. Note that we must clear
			// matched stop points since there could be user defined stop points
			// at the address, all of which are disabled.
			db.status.TrapKind = SingleStepTrap
			db.status.StopPoints = nil
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

func (db *Debugger) stepInstruction(stepOverCall bool) error {
	if stepOverCall {
		instructions, err := db.Disassemble(db.status.NextInstructionAddress, 1)
		if err != nil {
			return fmt.Errorf("failed to determine instruction type: %w", err)
		}

		// NOTE: golang's x86asm package is unable to disassemble all x64
		// instructions. When that happens, we'll simply assume the instruction is
		// not a call instruction.
		if len(instructions) == 1 {
			inst := instructions[0]
			if inst.Op == x86asm.CALL {
				return db.resumeUntilAddressOrSignal(
					db.status.NextInstructionAddress + VirtualAddress(inst.Len))
			}
		}
	}

	err := db.tracer.SingleStep()
	if err != nil {
		return fmt.Errorf("failed to single step: %w", err)
	}

	err = db.waitForSignal()
	if err != nil {
		return fmt.Errorf("failed to wait for step instruction: %w", err)
	}

	return nil
}

func (db *Debugger) waitForSignal() error {
	// NOTE: This returns on all traps, including traps on syscall that we
	// don't care about.
	waitStatus, err := db.signal.FromProcess()
	if err != nil {
		return err
	}

	return db.updateStatus(waitStatus)
}

func (db *Debugger) shouldUpdateSharedLibraries(status *ProcessStatus) bool {
	if db.LoadedElves.EntryPoint() == status.NextInstructionAddress {
		return true
	}

	if db.rendezvousNotifySite != nil &&
		db.rendezvousNotifySite.Address() == status.NextInstructionAddress {

		return true
	}

	if !db.ownsProcess && db.rendezvousNotifySite == nil {
		// When attaching to an existing process, the process' may have already
		// moved pass its entry point.  In that case, attempt best effort check.
		symbol := db.LoadedElves.SymbolSpans(status.NextInstructionAddress)
		return symbol != nil && symbol.Type() == elf.SymbolTypeFunction
	}

	return false
}

func (db *Debugger) updateSharedLibraries() error {
	notifyAddress, modified, err := db.LoadedElves.UpdateFiles()
	if err != nil {
		if errors.Is(err, ErrRendezvousAddressNotFound) {
			return nil
		}
		return err
	}

	if db.rendezvousNotifySite == nil {
		site, err := db.stopSites.Allocate(
			notifyAddress,
			stoppoint.NewBreakSiteType(false))
		if err != nil {
			return err
		}

		err = site.Enable()
		if err != nil {
			return err
		}

		db.rendezvousNotifySite = site
		db.rendezvousAddresses[notifyAddress] = struct{}{}
	}

	if modified {
		err := db.BreakPoints.ResolveStopSites()
		if err != nil {
			return err
		}

		err = db.WatchPoints.ResolveStopSites()
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *Debugger) updateStatus(waitStatus syscall.WaitStatus) error {
	// NOTE: we will immediately update the db.status with a simple status since
	// Close needs accurate state to clean up properly.  We'll supplement this
	// with a detailed status whenever possible, which provide debug information
	// to the user.
	db.status = newSimpleWaitingStatus(db.Pid, waitStatus)

	status, shouldResetProgramCounter, err := newDetailedWaitingStatus(
		db,
		waitStatus)
	if err != nil {
		return fmt.Errorf("failed to wait for process %d: %w", db.Pid, err)
	}

	if shouldResetProgramCounter {
		err := db.Registers.SetProgramCounter(status.NextInstructionAddress)
		if err != nil {
			return fmt.Errorf(
				"failed to wait for process %d. "+
					"cannot reset program counter at break point: %w",
				db.Pid,
				err)
		}
	}

	if status.Stopped {
		if db.shouldUpdateSharedLibraries(status) {
			err = db.updateSharedLibraries()
			if err != nil {
				return fmt.Errorf("failed to update shared libs: %w", err)
			}
		}

		state, err := db.Registers.GetState()
		if err != nil {
			return fmt.Errorf(
				"failed to update call stack for process %d: %w",
				db.Pid,
				err)
		}

		err = db.CallStack.Update(status, state)
		if err != nil {
			return fmt.Errorf(
				"failed to update call stack for process %d: %w",
				db.Pid,
				err)
		}

		if status.StopSignal == syscall.SIGTRAP {
			if status.TrapKind == SyscallTrap {
				db.expectsSyscallExit = !db.expectsSyscallExit
			} else {
				// In case syscall catch point got disabled after syscall entry, but
				// before syscall exit.
				db.expectsSyscallExit = false
			}
		}
	}

	db.status = status
	return nil
}

func (db *Debugger) Close() error {
	defer func() {
		_ = db.signal.Close()
	}()

	if db.status.Running() {
		err := db.signal.StopToProcess()
		if err != nil {
			return err
		}

		err = db.waitForSignal()
		if err != nil {
			return err
		}
	}

	if db.status.Exited { // nothing to detach from
		return nil
	}

	err := db.tracer.Detach()
	if err != nil {
		return err
	}

	err = db.signal.ContinueToProcess()
	if err != nil {
		return err
	}

	if db.ownsProcess {
		err = db.signal.KillToProcess()
		if err != nil {
			return err
		}

		err = db.waitForSignal()
		if err != nil {
			return err
		}
	}

	return nil
}
