package debugger

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"syscall"

	"github.com/pattyshack/bad/debugger/catchpoint"
	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/loadedelves"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/debugger/registers"
	"github.com/pattyshack/bad/debugger/stoppoint"
	"github.com/pattyshack/bad/elf"
	"github.com/pattyshack/bad/procfs"
	"github.com/pattyshack/bad/ptrace"
)

type Debugger struct {
	Pid           int
	ownsProcess   bool
	processTracer *ptrace.Tracer

	signal *Signaler

	LoadedElves *loadedelves.Files
	*SourceFiles

	VirtualMemory *memory.VirtualMemory
	*memory.Disassembler

	stopSites stoppoint.StopSitePool

	stoppoint.StopSiteResolverFactory

	BreakPoints *stoppoint.StopPointSet
	WatchPoints *stoppoint.StopPointSet

	SyscallCatchPolicy *catchpoint.SyscallCatchPolicy

	entryPointSite       stoppoint.StopSite
	rendezvousNotifySite stoppoint.StopSite
	rendezvousAddresses  map[VirtualAddress]struct{}

	currentTid int
	threads    map[int]*ThreadState

	threadLifeCycleWatchers []func(*ThreadStatus)
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

func newDebugger(
	processTracer *ptrace.Tracer,
	ownsProcess bool,
) (
	*Debugger,
	error,
) {
	mem := memory.New(processTracer)
	loadedElves := loadedelves.NewFiles(mem)

	db := &Debugger{
		Pid:                     processTracer.Pid,
		ownsProcess:             ownsProcess,
		processTracer:           processTracer,
		signal:                  NewSignaler(processTracer.Pid),
		LoadedElves:             loadedElves,
		SourceFiles:             NewSourceFiles(),
		VirtualMemory:           mem,
		StopSiteResolverFactory: stoppoint.NewStopSiteResolverFactory(loadedElves),
		SyscallCatchPolicy:      catchpoint.NewSyscallCatchPolicy(),
		rendezvousAddresses:     map[VirtualAddress]struct{}{},
		currentTid:              processTracer.Pid,
		threads:                 map[int]*ThreadState{},
	}

	stopSites := stoppoint.NewStopSitePool(db)

	db.stopSites = stopSites
	db.BreakPoints = stoppoint.NewBreakPointSet(stopSites)
	db.WatchPoints = stoppoint.NewWatchPointSet(stopSites)
	db.Disassembler = memory.NewDisassembler(mem, stopSites)

	if !ownsProcess {
		// Sig stop the process to prevent threads creation / termination while
		// setting up thread states.
		err := db.signal.StopToProcess()
		if err != nil {
			return nil, fmt.Errorf("failed to stop process %d: %w", db.Pid, err)
		}
	}

	mainWaitStatus, err := db.signal.FromThread(db.Pid)
	if err != nil {
		_ = processTracer.Close()
		return nil, fmt.Errorf(
			"failed to wait for main thread %d: %w",
			db.Pid,
			err)
	}

	// LoadBinary must be called after main thread stopped to avoid procfs
	// data race (the debugger could read procfs before the process entry point
	// address is written to procfs)
	_, err = db.LoadedElves.LoadExecutable(db.Pid)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	// Any thread created prior to this point (including the main thread)
	// should be listed in procfs and must be explicitly ptrace attached.
	existingTids, err := procfs.ListTasks(db.Pid)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf(
			"failed to list running threads for process %d: %w",
			db.Pid,
			err)
	}

	options := ptrace.O_TRACESYSGOOD | ptrace.O_TRACECLONE
	if ownsProcess {
		options |= ptrace.O_EXITKILL
	}

	for _, tid := range existingTids {
		var threadTracer *ptrace.Tracer
		var waitStatus syscall.WaitStatus
		var err error
		if tid == db.Pid {
			threadTracer = processTracer.TraceThread(db.Pid)
			waitStatus = mainWaitStatus
		} else {
			// NOTE: threads created prior to ptrace attaching to the main thread
			// are treated as independent tasks, and must be manually attached.
			threadTracer, err = ptrace.AttachToProcess(tid)
			if err != nil {
				_ = db.Close()
				return nil, fmt.Errorf(
					"failed to ptrace attach to thread %d: %w",
					tid,
					err)
			}

			waitStatus, err = db.signal.FromThread(tid)
			if err != nil {
				_ = threadTracer.Close()
				_ = db.Close()
				return nil, fmt.Errorf(
					"failed to wait for thread %d: %w",
					tid,
					err)
			}
		}

		thread, err := db.addThread(tid, threadTracer, waitStatus)
		if err != nil {
			_ = db.Close()
			return nil, err
		}

		// We need to account for the above explicit sig stop
		thread.hasPendingSigStop = !ownsProcess

		err = threadTracer.SetOptions(options)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf(
				"failed to set ptrace options for process %d: %w",
				processTracer.Pid,
				err)
		}
	}

	db.signal.ForwardInterruptToProcess()

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

	return db, nil
}

func (db *Debugger) Close() error {
	defer func() {
		_ = db.signal.Close()
		_ = db.processTracer.Close()
	}()

	if db.MainThread().status.Running() {
		err := db.signal.StopToProcess()
		if err != nil {
			return err
		}

		_, err = db.signal.FromThread(db.Pid)
		if err != nil {
			return err
		}
	}

	if db.MainThread().status.Exited {
		return nil
	}

	err := db.processTracer.Detach()
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
	}

	return nil
}

func (db *Debugger) ListThreads() (*ThreadState, []*ThreadState) {
	threads := []*ThreadState{}
	for _, thread := range db.threads {
		threads = append(threads, thread)
	}

	sort.Slice(
		threads,
		func(i int, j int) bool {
			return threads[i].Tid < threads[j].Tid
		})

	return db.CurrentThread(), threads
}

func (db *Debugger) SetCurrentThread(tid int) error {
	_, ok := db.threads[tid]
	if !ok {
		return fmt.Errorf("%w. no such thread", ErrInvalidArgument)
	}

	db.currentTid = tid
	return nil
}

func (db *Debugger) CurrentThread() *ThreadState {
	return db.threads[db.currentTid]
}

func (db *Debugger) MainThread() *ThreadState {
	return db.threads[db.Pid]
}

func (db *Debugger) WatchThreadLifeCycle(notify func(*ThreadStatus)) {
	db.threadLifeCycleWatchers = append(db.threadLifeCycleWatchers, notify)
}

func (db *Debugger) Memory() *memory.VirtualMemory {
	return db.VirtualMemory
}

func (db *Debugger) AllRegisters() []*registers.Registers {
	all := []*registers.Registers{}
	for _, thread := range db.threads {
		if thread.status.Exited || thread.status.Signaled {
			continue
		}

		all = append(all, thread.Registers)
	}
	return all
}

func (db *Debugger) ResumeCurrentUntilSignal() (*ThreadStatus, error) {
	thread := db.CurrentThread()
	if db.MainThread().status.Exited {
		return nil, fmt.Errorf(
			"failed to resume thread %d: %w",
			thread.Tid,
			ErrProcessExited)
	}

	err := thread.maybeBypassCurrentPCBreakSite()
	if err != nil {
		return nil, err
	}

	// Note that the current thread may have been updated by resumeUntilSignal.
	status, err := db.resumeUntilSignal(thread)
	if err != nil {
		return nil, err
	}

	return status, nil
}

func (db *Debugger) ResumeAllUntilSignal() (*ThreadStatus, error) {
	if db.MainThread().status.Exited {
		return nil, fmt.Errorf("failed to resume all threads: %w", ErrProcessExited)
	}

	// Ensure all threads have advance by at least one instruction
	for _, thread := range db.threads {
		err := thread.maybeBypassCurrentPCBreakSite()
		if err != nil {
			return nil, err
		}
	}

	// Note that the current thread may have been updated by resumeUntilSignal.
	status, err := db.resumeUntilSignal(nil)
	if err != nil {
		return nil, err
	}

	return status, nil
}

func (db *Debugger) StepInstruction() (*ThreadStatus, error) {
	thread := db.CurrentThread()
	if db.MainThread().status.Exited {
		return nil, fmt.Errorf(
			"failed to step instruction for thread %d: %w",
			thread.Tid,
			ErrProcessExited)
	}

	err := thread.maybeSwallowInternalSigStop()
	if err != nil {
		return nil, err
	}

	err = thread.stepInstruction(true, false)
	if err != nil {
		return nil, err
	}

	reportStatus := db.focusOnImportantStatus(thread, nil)
	if reportStatus != nil {
		return reportStatus, nil
	}

	return thread.status, nil
}

func (db *Debugger) StepIn() (*ThreadStatus, error) {
	thread := db.CurrentThread()
	if db.MainThread().status.Exited {
		return nil, fmt.Errorf(
			"failed to step in for thread %d: %w",
			thread.Tid,
			ErrProcessExited)
	}

	inlinedStepInStatus, err := thread.CallStack.MaybeStepIntoInlinedFunction(
		thread.status)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to step in for thread %d: %w",
			thread.Tid,
			err)
	}

	if inlinedStepInStatus != nil {
		thread.status = inlinedStepInStatus
		thread.expectsSyscallExit = false
		return thread.status, nil
	}

	err = thread.maybeSwallowInternalSigStop()
	if err != nil {
		return nil, err
	}

	err = thread.stepUntilDifferentLine(false)
	if err != nil {
		return nil, err
	}

	err = thread.maybeStepOverFunctionPrologue()
	if err != nil {
		return nil, err
	}

	reportStatus := db.focusOnImportantStatus(thread, nil)
	if reportStatus != nil {
		return reportStatus, nil
	}

	return thread.status, nil
}

func (db *Debugger) StepOver() (*ThreadStatus, error) {
	thread := db.CurrentThread()
	if db.MainThread().status.Exited {
		return nil, fmt.Errorf(
			"failed to step over for thread %d: %w",
			thread.Tid,
			ErrProcessExited)
	}

	err := thread.maybeSwallowInternalSigStop()
	if err != nil {
		return nil, err
	}

	err = thread.stepUntilDifferentLine(true)
	if err != nil {
		return nil, err
	}

	reportStatus := db.focusOnImportantStatus(thread, nil)
	if reportStatus != nil {
		return reportStatus, nil
	}

	return thread.status, nil
}

func (db *Debugger) StepOut() (*ThreadStatus, error) {
	thread := db.CurrentThread()
	if db.MainThread().status.Exited {
		return nil, fmt.Errorf(
			"failed to step out for thread %d: %w",
			thread.Tid,
			ErrProcessExited)
	}

	err := thread.maybeSwallowInternalSigStop()
	if err != nil {
		return nil, err
	}

	var returnAddress VirtualAddress
	frame := thread.CallStack.CurrentFrame()
	if frame != nil && frame.IsInlined() {
		// XXX: This is not completely correct since the inlined function may
		// jump to any address, but is good enough for our purpose.
		returnAddress = frame.CodeRanges[len(frame.CodeRanges)-1].High
	} else {
		state, err := thread.Registers.GetState()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to step out for thread %d: %w",
				thread.Tid,
				err)
		}

		framePointer := VirtualAddress(
			state.Value(registers.FramePointer).ToUint64())

		addressBytes := make([]byte, 8)
		n, err := db.VirtualMemory.Read(framePointer+8, addressBytes)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to step out for thread %d: %w",
				thread.Tid,
				err)
		}
		if n != 8 {
			panic("should never happen")
		}

		n, err = binary.Decode(addressBytes, binary.LittleEndian, &returnAddress)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to step out for thread %d: %w",
				thread.Tid,
				err)
		}
		if n != 8 {
			panic("should never happen")
		}
	}

	err = thread.stepInstruction(true, false)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to step out for thread %d: %w",
			thread.Tid,
			err)
	}

	if thread.status.Stopped &&
		thread.status.NextInstructionAddress != returnAddress {

		err = thread.resumeUntilAddressOrSignal(returnAddress)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to step out for thread %d: %w",
				thread.Tid,
				err)
		}
	}

	reportStatus := db.focusOnImportantStatus(thread, nil)
	if reportStatus != nil {
		return reportStatus, nil
	}

	return thread.status, nil
}

func (db *Debugger) addThread(
	tid int,
	threadTracer *ptrace.Tracer,
	waitStatus syscall.WaitStatus,
) (
	*ThreadState,
	error,
) {
	_, ok := db.threads[tid]
	if ok {
		return nil, fmt.Errorf("cannot add thread. thread %d already exist", tid)
	}

	thread := &ThreadState{
		Tid:          tid,
		threadTracer: threadTracer,
		Registers:    registers.New(threadTracer),
		CallStack:    newCallStack(db.LoadedElves, db.VirtualMemory),
		status:       newRunningStatus(tid),
		Debugger:     db,
	}
	db.threads[tid] = thread

	err := thread.updateStatus(waitStatus, true)
	if err != nil {
		return nil, err
	}

	for _, notify := range db.threadLifeCycleWatchers {
		notify(thread.status)
	}

	return thread, nil
}

func (db *Debugger) removeThread(tid int) error {
	thread, ok := db.threads[tid]
	if !ok {
		return fmt.Errorf("cannot remove thread %d. no such thread", tid)
	}

	err := thread.threadTracer.Detach()
	if err != nil {
		return err
	}

	delete(db.threads, tid)

	for _, notify := range db.threadLifeCycleWatchers {
		notify(thread.status)
	}

	return nil
}

func (db *Debugger) shouldUpdateSharedLibraries(
	status *ThreadStatus,
) bool {
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

		_, entries, err := db.LoadedElves.ReadRendezvousInfo()
		if err == nil {
			_, ok := entries[loadedelves.VDSOFileName]
			if ok {
				return true
			}

			addr, ok := entries[""] // executable
			if ok && uint64(addr) == db.LoadedElves.Executable.LoadBias {
				return true
			}
		}

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

func (db *Debugger) _stopRunningThreads(
	stopped map[int]syscall.WaitStatus,
) error {
	numRunning := 0
	for tid, thread := range db.threads {
		if !thread.status.Running() {
			continue
		}

		_, ok := stopped[tid]
		if ok {
			// thread has already stopped, but thread status has not been updated.
			continue
		}

		numRunning += 1

		if !thread.hasPendingSigStop && !thread.hasPendingSingleStepTrap {
			err := db.signal.StopToThread(tid)
			if err != nil {
				return err
			}
			thread.hasPendingSigStop = true
		}
	}

	for numRunning > 0 {
		tid, waitStatus, err := db.signal.FromProcessThreads()
		if err != nil {
			return err
		}

		_, ok := stopped[tid]
		if ok {
			panic("should never happen")
		}

		stopped[tid] = waitStatus

		thread, ok := db.threads[tid]
		if !ok { // new thread
			continue
		}

		if !thread.status.Running() {
			panic("should never happen")
		}

		numRunning -= 1
	}

	return nil
}

func (db *Debugger) _updateStoppedThreads(
	stopped map[int]syscall.WaitStatus,
) (
	map[int]*ThreadState,
	error,
) {
	shouldRefresh := false
	stoppedThreads := map[int]*ThreadState{}
	for tid, waitStatus := range stopped {
		thread, ok := db.threads[tid]
		if !ok {
			var err error
			thread, err = db.addThread(
				tid,
				db.processTracer.TraceThread(tid),
				waitStatus)
			if err != nil {
				return nil, err
			}

			shouldRefresh = true
		} else {
			err := thread.updateStatus(waitStatus, !ok)
			if err != nil {
				return nil, err
			}
		}

		if thread.status.Stopped {
			stoppedThreads[tid] = thread
		} else if thread.Tid != thread.Pid {
			err := db.removeThread(tid)
			if err != nil {
				return nil, err
			}

			shouldRefresh = true
		}
	}

	if shouldRefresh {
		err := db.stopSites.RefreshSites()
		if err != nil {
			return nil, err
		}
	}

	return stoppedThreads, nil
}

func (db *Debugger) waitForSignalFromAnyThread() (
	map[int]*ThreadState,
	error,
) {
	tid, waitStatus, err := db.signal.FromProcessThreads()
	if err != nil {
		return nil, err
	}

	stopped := map[int]syscall.WaitStatus{
		tid: waitStatus,
	}

	err = db._stopRunningThreads(stopped)
	if err != nil {
		return nil, err
	}

	stoppedThreads, err := db._updateStoppedThreads(stopped)
	if err != nil {
		return nil, err
	}

	return stoppedThreads, err
}

// Resume all when thread is nil.  Otherwise only resume the specified thread.
func (db *Debugger) resumeUntilSignal(
	resumeThread *ThreadState,
) (
	*ThreadStatus,
	error,
) {
	resume := func() error {
		resumeThreads := []*ThreadState{}
		if resumeThread != nil {
			resumeThreads = append(resumeThreads, resumeThread)
		} else {
			for _, thread := range db.threads {
				resumeThreads = append(resumeThreads, thread)
			}
		}

		// NOTE: all rendezvous must be stepped over before resuming threads since
		// the resumed threads may accidently bypass temporarily disabled sites.
		for _, thread := range resumeThreads {
			if thread.status.TrapKind == RendezvousTrap {
				err := thread.stepInstruction(true, false)
				if err != nil {
					return fmt.Errorf(
						"failed to resume until signal. "+
							"cannot step over rendezvous for thread %d: %w",
						thread.Tid,
						err)
				}
			}
		}

		for _, thread := range resumeThreads {
			err := thread.resume()
			if err != nil {
				return fmt.Errorf(
					"failed to resume until signal. cannot resume thread %d: %w",
					thread.Tid,
					err)
			}
		}

		return nil
	}

	for {
		err := resume()
		if err != nil {
			return nil, err
		}

		stoppedThreads, err := db.waitForSignalFromAnyThread()
		if err != nil {
			return nil, err
		}

		reportStatus := db.focusOnImportantStatus(resumeThread, stoppedThreads)
		if reportStatus != nil {
			return reportStatus, nil
		}
	}
}

// This returns a status if the focus shifted.  Otherwise this returns nil.
func (db *Debugger) focusOnImportantStatus(
	resumeThread *ThreadState, // nil for resume all
	stoppedThreads map[int]*ThreadState,
) *ThreadStatus {
	for _, thread := range stoppedThreads {
		if thread.status.IsInternalSigStop {
			continue
		}

		if thread.status.StopSignal != syscall.SIGTRAP {
			db.currentTid = thread.Tid
			return thread.status
		}

		switch thread.status.TrapKind {
		case SyscallTrap:
			if db.SyscallCatchPolicy.Matches(
				thread.status.SyscallTrapInfo.Id) {

				db.currentTid = thread.Tid
				return thread.status
			}
		case RendezvousTrap, CloneTrap:
			// do nothing
		default:
			db.currentTid = thread.Tid
			return thread.status
		}
	}

	// Also return if the non-main resumeThread exited / terminated (not listed
	// in stoppedThreads) to give user a chance to focus on another thread.
	if resumeThread != nil && !resumeThread.status.Stopped {
		if resumeThread.status.Running() {
			panic("should never happen")
		}

		// arbitrarily pick main thread since it's always available.
		db.currentTid = db.Pid
		return db.MainThread().status
	}

	if !db.MainThread().status.Stopped { // main thread exited / terminated
		db.currentTid = db.Pid
		return db.MainThread().status
	}

	return nil
}
