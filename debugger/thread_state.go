package debugger

import (
	"encoding/binary"
	"fmt"
	"math"
	"syscall"

	"golang.org/x/arch/x86/x86asm"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/expression"
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

	*Debugger
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

		err = thread.CallStack.Update(status)
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
		} else if status.StopSignal != syscall.SIGTRAP ||
			thread.Tid != thread.Pid {
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
	err := thread.maybeSwallowInternalSigStop()
	if err != nil {
		return err
	}

	var enabledSites stoppoint.StopSites
	if bypassEnabledSitesAtCurrentPC {
		enabledSites = thread.stopSites.GetEnabledAt(
			thread.status.NextInstructionAddress)
	}
	err = enabledSites.Disable()
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
	_, funcEntry, err := thread.LoadedElves.
		FunctionDefinitionEntryContainingAddress(pc)
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
	err := thread.maybeSwallowInternalSigStop()
	if err != nil {
		return err
	}

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

func (thread *ThreadState) ResumeUntilSignal() (*ThreadStatus, error) {
	if thread.Exited() {
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
	status, err := thread.resumeUntilSignal(thread)
	if err != nil {
		return nil, err
	}

	return status, nil
}

func (thread *ThreadState) StepInstruction() (*ThreadStatus, error) {
	if thread.Exited() {
		return nil, fmt.Errorf(
			"failed to step instruction for thread %d: %w",
			thread.Tid,
			ErrProcessExited)
	}

	err := thread.stepInstruction(true, false)
	if err != nil {
		return nil, err
	}

	reportStatus := thread.focusOnImportantStatus(thread, nil)
	if reportStatus != nil {
		return reportStatus, nil
	}

	return thread.status, nil
}

func (thread *ThreadState) StepIn() (*ThreadStatus, error) {
	if thread.Exited() {
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

	err = thread.stepUntilDifferentLine(false)
	if err != nil {
		return nil, err
	}

	err = thread.maybeStepOverFunctionPrologue()
	if err != nil {
		return nil, err
	}

	reportStatus := thread.focusOnImportantStatus(thread, nil)
	if reportStatus != nil {
		return reportStatus, nil
	}

	return thread.status, nil
}

func (thread *ThreadState) StepOver() (*ThreadStatus, error) {
	if thread.Exited() {
		return nil, fmt.Errorf(
			"failed to step over for thread %d: %w",
			thread.Tid,
			ErrProcessExited)
	}

	err := thread.stepUntilDifferentLine(true)
	if err != nil {
		return nil, err
	}

	reportStatus := thread.focusOnImportantStatus(thread, nil)
	if reportStatus != nil {
		return reportStatus, nil
	}

	return thread.status, nil
}

func (thread *ThreadState) StepOut() (*ThreadStatus, error) {
	if thread.Exited() {
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
	frame := thread.CallStack.ExecutingFrame()
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
		n, err := thread.VirtualMemory.Read(framePointer+8, addressBytes)
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

	reportStatus := thread.focusOnImportantStatus(thread, nil)
	if reportStatus != nil {
		return reportStatus, nil
	}

	return thread.status, nil
}

func (thread *ThreadState) InvokeMalloc(size int) (VirtualAddress, error) {
	malloc, err := thread.descriptorPool.GetMalloc()
	if err != nil {
		return 0, err
	}

	result, err := thread.Invoke(
		malloc,
		[]*expression.TypedData{
			thread.descriptorPool.NewInt32(fmt.Sprintf("%d", size), int32(size)),
		})
	if err != nil {
		return 0, err
	}

	address, err := result.DecodeSimpleValue()
	if err != nil {
		return 0, err
	}

	return address.(VirtualAddress), nil
}

func (thread *ThreadState) Invoke(
	functionOrMethod *expression.TypedData,
	arguments []*expression.TypedData,
) (
	*expression.TypedData,
	error,
) {
	signature, funcAddr, err := functionOrMethod.SelectMatchingSignature(
		arguments)
	if err != nil {
		return nil, err
	}

	var retValAddr VirtualAddress
	if signature.ReturnInMemory || !signature.Return.IsSimpleValue() {

		retValAddr, err = thread.InvokeMalloc(signature.Return.ByteSize)
		if err != nil {
			return nil, err
		}
	}

	entryPointSite, err := thread.stopSites.Allocate(
		thread.LoadedElves.EntryPoint(),
		stoppoint.NewBreakSiteType(false))
	if err != nil {
		return nil, err
	}

	err = entryPointSite.Enable()
	if err != nil {
		return nil, err
	}

	originalStatus := thread.status
	originalCallStack := *thread.CallStack
	originalState, err := thread.setupRegistersAndStackForCall(
		signature,
		funcAddr,
		functionOrMethod,
		arguments,
		entryPointSite.Address(),
		retValAddr)
	if err != nil {
		return nil, err
	}

	// NOTE: for simplicity, we assume that invoke is not interruptible by
	// breakpoints, etc.
	for {
		_, err = thread.ResumeUntilSignal()
		if err != nil {
			return nil, err
		}

		if !thread.status.Stopped {
			return nil, fmt.Errorf(
				"thread unexpectedly exited during function invocation:\n%v",
				thread.status)
		}

		if thread.status.NextInstructionAddress == entryPointSite.Address() {
			break
		}
	}

	returnValue, err := thread.readReturnValueForCall(signature, retValAddr)
	if err != nil {
		return nil, err
	}

	err = thread.Registers.SetState(originalState)
	if err != nil {
		return nil, err
	}
	thread.status = originalStatus
	thread.CallStack = &originalCallStack

	err = entryPointSite.Deallocate()
	if err != nil {
		return nil, err
	}

	return returnValue, nil
}

func (thread *ThreadState) setupRegistersAndStackForCall(
	signature *expression.SignatureDescriptor,
	funcAddr VirtualAddress,
	functionOrMethod *expression.TypedData,
	explicitArgs []*expression.TypedData,
	returnAddr VirtualAddress,
	retValAddr VirtualAddress,
) (
	registers.State,
	error,
) {
	originalState, err := thread.Registers.GetState()
	if err != nil {
		return registers.State{}, err
	}

	invokeState, err := originalState.WithValue(
		registers.ProgramCounter,
		registers.U64(uint64(funcAddr)))
	if err != nil {
		return registers.State{}, err
	}

	if signature.ReturnInMemory {
		rdi, ok := registers.ByName("rdi")
		if !ok {
			panic("should never happen")
		}

		invokeState, err = invokeState.WithValue(
			rdi,
			registers.U64(uint64(retValAddr)))
	}

	stackPointer := originalState.Value(registers.StackPointer).ToUint64()

	// Reserve stack space for arguments. SYS V ABI requires 16 byte aligned
	// address for the top stack argument.
	stackPointer -= signature.ParameterStackSize
	stackPointer = (stackPointer / 16) * 16

	stackPointer -= 8 // Space for return address

	invokeState, err = invokeState.WithValue(
		registers.StackPointer,
		registers.U64(stackPointer))

	returnAddrBytes := make([]byte, 8)
	n, err := binary.Encode(returnAddrBytes, binary.LittleEndian, returnAddr)
	if err != nil {
		return registers.State{}, err
	}
	if n != 8 {
		panic("should never happen")
	}

	n, err = thread.VirtualMemory.Write(
		VirtualAddress(stackPointer),
		returnAddrBytes)
	if err != nil {
		return registers.State{}, err
	}
	if n != 8 {
		panic("should never happen")
	}

	// SYS V ABI requires us to save the number of SSE registers used in rax to
	// keep track of varargs.
	rax, ok := registers.ByName("rax")
	if !ok {
		panic("should never happen")
	}

	invokeState, err = invokeState.WithValue(
		rax,
		registers.U64(signature.NumSSERegistersUsed))
	if err != nil {
		return registers.State{}, err
	}

	arguments := explicitArgs
	if functionOrMethod.Kind == expression.MethodKind {
		receiver := functionOrMethod.MethodReceiverPointer(signature)
		arguments = append([]*expression.TypedData{receiver}, explicitArgs...)
	}

	for paramIdx, param := range signature.Parameters {
		arg := arguments[paramIdx]
		data, err := arg.Bytes()
		if err != nil {
			return registers.State{}, err
		}

		if param.StackOffset != 0 {
			n, err = thread.VirtualMemory.Write(
				VirtualAddress(stackPointer+param.StackOffset),
				data)
			if err != nil {
				return registers.State{}, err
			}
			if n != len(data) {
				panic("should never happen")
			}
		} else {
			// pad data until length is 8 byte aligned
			for len(data)%8 != 0 {
				data = append(data, 0)
			}

			for classIdx, registerName := range param.Registers {
				start := classIdx * 8
				end := start + 8

				value := uint64(0)
				n, err = binary.Decode(data[start:end], binary.LittleEndian, &value)
				if err != nil {
					return registers.State{}, err
				}
				if n != 8 {
					panic("should never happen")
				}

				register, ok := registers.ByName(registerName)
				if !ok {
					panic("should never happen")
				}

				if register.Class == registers.GeneralClass {
					invokeState, err = invokeState.WithValue(
						register,
						registers.U64(value))
				} else {
					invokeState, err = invokeState.WithValue(
						register,
						registers.U128(0, value))
				}
				if err != nil {
					return registers.State{}, err
				}
			}
		}
	}

	err = thread.Registers.SetState(invokeState)
	if err != nil {
		return registers.State{}, err
	}

	return originalState, nil
}

func (thread *ThreadState) readReturnValueForCall(
	signature *expression.SignatureDescriptor,
	retValAddr VirtualAddress,
) (
	*expression.TypedData,
	error,
) {
	if signature.Return.Kind == expression.VoidKind {
		return thread.descriptorPool.NewVoid(), nil
	}

	retVal := &expression.TypedData{
		VirtualMemory:  thread.VirtualMemory,
		FormatPrefix:   "(call)",
		DataDescriptor: signature.Return,
	}

	if signature.ReturnInMemory {
		retVal.Address = retValAddr
		retVal.BitSize = signature.Return.ByteSize * 8

		return retVal, nil
	}

	state, err := thread.Registers.GetState()
	if err != nil {
		return nil, err
	}

	uint64Data := []uint64{}
	for _, registerName := range signature.ReturnOnRegisters {
		register, ok := registers.ByName(registerName)
		if !ok {
			panic("should never happen")
		}

		value := state.Value(register)
		if register.Class == registers.FloatingPointClass {
			u128 := value.ToUint128()
			uint64Data = append(uint64Data, u128.Low, u128.High)
		} else { // general class
			uint64Data = append(uint64Data, value.ToUint64())
		}
	}

	if signature.Return.IsSimpleValue() {
		expected := 1
		if signature.Return.Kind == expression.FloatKind {
			expected = 2
		}

		if len(uint64Data) != expected {
			panic("should never happen")
		}

		value := uint64Data[0]

		switch signature.Return.Kind {
		case expression.BoolKind:
			retVal.ImplicitValue = value != 0
		case expression.CharKind:
			retVal.ImplicitValue = byte(uint8(value))
		case expression.PointerKind:
			retVal.ImplicitValue = VirtualAddress(value)
		case expression.IntKind:
			switch signature.Return.ByteSize {
			case 1:
				retVal.ImplicitValue = int8(value)
			case 2:
				retVal.ImplicitValue = int16(value)
			case 4:
				retVal.ImplicitValue = int32(value)
			case 8:
				retVal.ImplicitValue = int64(value)
			default:
				return nil, fmt.Errorf("unsupported int size")
			}
		case expression.UintKind:
			switch signature.Return.ByteSize {
			case 1:
				retVal.ImplicitValue = uint8(value)
			case 2:
				retVal.ImplicitValue = uint16(value)
			case 4:
				retVal.ImplicitValue = uint32(value)
			case 8:
				retVal.ImplicitValue = value
			default:
				return nil, fmt.Errorf("unsupported uint size")
			}
		case expression.FloatKind:
			switch signature.Return.ByteSize {
			case 4:
				retVal.ImplicitValue = math.Float32frombits(uint32(value))
			case 8:
				retVal.ImplicitValue = math.Float64frombits(value)
			default:
				return nil, fmt.Errorf("unsupported float size")
			}
		default:
			panic("should never happen")
		}

		return retVal, nil
	}

	data := make([]byte, len(uint64Data)*8)
	for idx, value := range uint64Data {
		n, err := binary.Encode(data[idx*8:], binary.LittleEndian, value)
		if err != nil {
			return nil, fmt.Errorf("cannot encode return value: %w", err)
		}
		if n != 8 {
			panic("should never happen")
		}
	}

	n, err := thread.VirtualMemory.Write(retValAddr, data[:retVal.ByteSize])
	if err != nil {
		return nil, fmt.Errorf("failed to copy return value: %w", err)
	}
	if n != retVal.ByteSize {
		panic("should never happen")
	}

	retVal.Address = retValAddr
	retVal.BitSize = signature.Return.ByteSize * 8

	return retVal, nil
}
