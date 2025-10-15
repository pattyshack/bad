package debugger

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"syscall"

	"github.com/pattyshack/bad/debugger/catchpoint"
	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/expression"
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

	descriptorPool *expression.DataDescriptorPool

	stopSites stoppoint.StopSitePool

	stoppoint.StopSiteResolverFactory

	BreakPoints *stoppoint.StopPointSet
	WatchPoints *stoppoint.StopPointSet

	SyscallCatchPolicy *catchpoint.SyscallCatchPolicy

	EvaluatedResults *expression.EvaluatedResultPool

	entryPointRendezvousSite stoppoint.StopSite
	rendezvousNotifySite     stoppoint.StopSite
	rendezvousAddresses      map[VirtualAddress]struct{}

	currentTid int
	threads    map[int]*ThreadState

	threadLifeCycleWatchers []func(*ThreadStatus)
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
		descriptorPool:          expression.NewDataDescriptorPool(loadedElves, mem),
		StopSiteResolverFactory: stoppoint.NewStopSiteResolverFactory(loadedElves),
		SyscallCatchPolicy:      catchpoint.NewSyscallCatchPolicy(),
		EvaluatedResults:        &expression.EvaluatedResultPool{},
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

	db.entryPointRendezvousSite = entryPointSite
	db.rendezvousAddresses[db.LoadedElves.EntryPoint()] = struct{}{}

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

func (db *Debugger) Close() error {
	defer func() {
		_ = db.signal.Close()
		_ = db.processTracer.Close()
	}()

	if db.mainThread().status.Running() {
		err := db.signal.StopToProcess()
		if err != nil {
			return err
		}

		_, err = db.signal.FromThread(db.Pid)
		if err != nil {
			return err
		}
	}

	if db.mainThread().status.Exited {
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

func (db *Debugger) WatchThreadLifeCycle(notify func(*ThreadStatus)) {
	db.threadLifeCycleWatchers = append(
		db.threadLifeCycleWatchers,
		notify)
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

	return db.currentThread(), threads
}

func (db *Debugger) SetCurrentThread(tid int) error {
	_, ok := db.threads[tid]
	if !ok {
		return fmt.Errorf("%w. no such thread", ErrInvalidInput)
	}

	db.currentTid = tid
	return nil
}

func (db *Debugger) currentThread() *ThreadState {
	return db.threads[db.currentTid]
}

func (db *Debugger) mainThread() *ThreadState {
	return db.threads[db.Pid]
}

func (db *Debugger) CurrentStatus() *ThreadStatus {
	return db.currentThread().Status()
}

func (db *Debugger) Exited() bool {
	return db.mainThread().status.Exited
}

func (db *Debugger) BacktraceStack() (*CallFrame, []*CallFrame) {
	stack := db.currentThread().CallStack
	return stack.CurrentInspectFrame(), stack.ExecutingStack()
}

func (db *Debugger) InspectCalleeFrame() {
	db.currentThread().CallStack.InspectCalleeFrame()
}

func (db *Debugger) InspectCallerFrame() {
	db.currentThread().CallStack.InspectCallerFrame()
}

func (db *Debugger) GetInspectFrameRegisterState() (registers.State, error) {
	return db.currentThread().CallStack.GetInspectFrameRegisterState()
}

func (db *Debugger) SetInspectFrameRegisterState(state registers.State) error {
	return db.currentThread().CallStack.SetInspectFrameRegisterState(
		state)
}

func (db *Debugger) ListInspectFrameLocalVariables() (
	[]*expression.TypedData,
	error,
) {
	return db.currentThread().CallStack.ListInspectFrameLocalVariables()
}

func (db *Debugger) ReadInspectFrameVariableOrFunction(
	name string,
) (
	*expression.TypedData,
	error,
) {
	return db.currentThread().CallStack.ReadInspectFrameVariableOrFunction(name)
}

func (db *Debugger) InvokeMallocInCurrentThread(
	size int,
) (
	VirtualAddress,
	error,
) {
	return db.currentThread().InvokeMalloc(size)
}

func (db *Debugger) InvokeInCurrentThread(
	functionOrMethod *expression.TypedData,
	arguments []*expression.TypedData,
) (
	*expression.TypedData,
	error,
) {
	return db.currentThread().Invoke(functionOrMethod, arguments)
}

func (db *Debugger) Memory() *memory.VirtualMemory {
	return db.VirtualMemory
}

func (db *Debugger) DescriptorPool() *expression.DataDescriptorPool {
	return db.descriptorPool
}

func (db *Debugger) GetEvaluatedResult(
	idx int,
) (
	*expression.EvaluatedResult,
	error,
) {
	return db.EvaluatedResults.Get(idx)
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
		status:       newRunningStatus(tid),
		Debugger:     db,
	}
	thread.CallStack = newCallStack(thread)
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

	if db.entryPointRendezvousSite != nil {
		delete(db.rendezvousAddresses, db.LoadedElves.EntryPoint())
		err := db.entryPointRendezvousSite.Deallocate()
		if err != nil {
			return err
		}
		db.entryPointRendezvousSite = nil
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
		} else if thread.Tid != db.Pid {
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
		return db.mainThread().status
	}

	if !db.mainThread().status.Stopped { // main thread exited / terminated
		db.currentTid = db.Pid
		return db.mainThread().status
	}

	return nil
}

func (db *Debugger) ResumeAllUntilSignal() (*ThreadStatus, error) {
	if db.Exited() {
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

func (db *Debugger) ResumeCurrentUntilSignal() (*ThreadStatus, error) {
	return db.currentThread().ResumeUntilSignal()
}

func (db *Debugger) StepInstruction() (*ThreadStatus, error) {
	return db.currentThread().StepInstruction()
}

func (db *Debugger) StepIn() (*ThreadStatus, error) {
	return db.currentThread().StepIn()
}

func (db *Debugger) StepOver() (*ThreadStatus, error) {
	return db.currentThread().StepOver()
}

func (db *Debugger) StepOut() (*ThreadStatus, error) {
	return db.currentThread().StepOut()
}

func (db *Debugger) ResolveVariableExpression(
	expressionString string,
) (
	*expression.EvaluatedResult,
	error,
) {
	value, err := expression.Evaluate(db, expressionString)
	if err != nil {
		return nil, err
	}

	if value.ImplicitValue != nil {
		addr, err := db.InvokeMallocInCurrentThread(value.ByteSize)
		if err != nil {
			return nil, err
		}

		data, err := value.Bytes()
		if err != nil {
			return nil, err
		}

		n, err := db.VirtualMemory.Write(addr, data)
		if err != nil {
			return nil, err
		}
		if n != len(data) {
			panic("should never happen")
		}

		value.Address = addr
		value.BitSize = value.ByteSize * 8
		value.ImplicitValue = nil
	}

	return db.EvaluatedResults.Save(expressionString, value), nil
}
