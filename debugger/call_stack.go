package debugger

import (
	"encoding/binary"
	"fmt"
	"sort"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/expression"
	"github.com/pattyshack/bad/debugger/loadedelves"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/debugger/registers"
	"github.com/pattyshack/bad/dwarf"
)

type CallFrame struct {
	// Inlined frame's base frame.
	BaseFrame *CallFrame

	File           *loadedelves.File
	DebugInfoEntry *dwarf.DebugInfoEntry

	Name       string
	CodeRanges AddressRanges

	SourceFile *dwarf.FileEntry
	SourceLine int64

	BacktraceProgramCounter VirtualAddress

	// Note the inlined functions will have the same register state as the base
	// function.
	//
	// For the currently executing (non-inlined) frame, Registers holds the
	// current register state (manual debugger register changes are ignored).
	// For other backtrace frames, Registers holds the restored values of what
	// the registers have if the call frame above it returned immediately.
	Registers registers.State

	memory *memory.VirtualMemory

	// NOTE: canonical frame address is only populated in the base frame.
	cfa registers.Value
}

func (frame *CallFrame) IsInlined() bool {
	return frame.BaseFrame != nil
}

func (CallFrame) ByteOrder() binary.ByteOrder {
	return binary.LittleEndian
}

func (frame *CallFrame) LoadBias() uint64 {
	return frame.File.LoadBias
}

func (frame *CallFrame) CurrentFunctionEntry() *dwarf.DebugInfoEntry {
	return frame.DebugInfoEntry
}

func (frame *CallFrame) BaseFrameFunctionEntry() *dwarf.DebugInfoEntry {
	if frame.BaseFrame != nil {
		return frame.BaseFrame.DebugInfoEntry
	}
	return frame.DebugInfoEntry
}

func (frame *CallFrame) ProgramCounter() uint64 {
	return uint64(frame.BacktraceProgramCounter)
}

func (frame *CallFrame) RegisterValue(
	id dwarf.RegisterId,
) (
	uint64,
	error,
) {
	spec, ok := registers.ById(id)
	if !ok {
		return 0, fmt.Errorf("invalid register id %d", id)
	}

	if spec.Size > 8 {
		return 0, fmt.Errorf("unsupported register size")
	}

	value := frame.Registers.Value(spec)
	if value == nil {
		return 0, fmt.Errorf("register (%d) value unavailable", id)
	}

	return value.ToUint64(), nil
}

func (frame *CallFrame) ReadMemory(
	virtualAddress uint64,
	out []byte,
) (
	int,
	error,
) {
	return frame.memory.Read(VirtualAddress(virtualAddress), out)
}

func (frame *CallFrame) CanonicalFrameAddress() (uint64, error) {
	cfa := frame.cfa
	if frame.BaseFrame != nil {
		cfa = frame.BaseFrame.cfa
	}

	if cfa == nil {
		return 0, fmt.Errorf("cfa unavailable")
	}

	return cfa.ToUint64(), nil
}

func (frame *CallFrame) readLocationData(
	location dwarf.Location,
	byteSize int,
) (
	[]byte,
	error,
) {
	appender := &BitsAppender{}

	for _, chunk := range location {
		var chunkData []byte
		switch chunk.Kind {
		case dwarf.RegisterLocation:
			id := dwarf.RegisterId(chunk.Value)
			spec, ok := registers.ById(id)
			if !ok {
				return nil, fmt.Errorf("invalid register id %d", id)
			}

			value := frame.Registers.Value(spec)
			if value == nil {
				return nil, fmt.Errorf("register (%d) value unavailable", id)
			}

			chunkData = value.ToBytes()
		case dwarf.AddressLocation:
			chunkData = make([]byte, byteSize)
			n, err := frame.memory.Read(VirtualAddress(chunk.Value), chunkData)
			if err != nil {
				return nil, err
			}

			chunkData = chunkData[:n]
		case dwarf.ImplicitLiteralLocation:
			chunkData = registers.U64(chunk.Value).ToBytes()
		case dwarf.ImplicitDataLocation:
			chunkData = chunk.Data
		default:
			return nil, fmt.Errorf(
				"data unavailable for location kind (%s)",
				chunk.Kind)
		}

		bitSize := int(chunk.BitSize)
		if bitSize == 0 {
			bitSize = len(chunkData) * 8
		}

		appender.AppendSlice(chunkData, int(chunk.BitOffset), bitSize)
	}

	return appender.Finalize(), nil
}

type CallStack struct {
	*ThreadState

	currentPC VirtualAddress

	// On update, initialized to the inner most inlined function frame with
	// low address < pc (or the outer most non-inlined function frame).
	executingFrame int

	// On update, reset to the executing frame.
	currentInspectFrame int

	// The first entry is the top of the call stack.
	frames []*CallFrame
}

func newCallStack(thread *ThreadState) *CallStack {
	return &CallStack{
		ThreadState:         thread,
		currentPC:           0,
		executingFrame:      0,
		currentInspectFrame: 0,
	}
}

func (stack *CallStack) InspectCalleeFrame() {
	if stack.currentInspectFrame > stack.executingFrame {
		stack.currentInspectFrame -= 1
	}
}

func (stack *CallStack) InspectCallerFrame() {
	if stack.currentInspectFrame < len(stack.frames)-1 {
		stack.currentInspectFrame += 1
	}
}

func (stack *CallStack) CurrentInspectFrame() *CallFrame {
	if len(stack.frames) > 0 {
		return stack.frames[stack.currentInspectFrame]
	}
	return nil
}

func (stack *CallStack) GetInspectFrameRegisterState() (
	registers.State,
	error,
) {
	if len(stack.frames) == 0 ||
		stack.currentInspectFrame == stack.executingFrame {

		return stack.Registers.GetState()
	}

	return stack.CurrentInspectFrame().Registers, nil
}

func (stack *CallStack) SetInspectFrameRegisterState(
	state registers.State,
) error {
	if len(stack.frames) == 0 ||
		stack.currentInspectFrame == stack.executingFrame {

		return stack.Registers.SetState(state)
	}

	return fmt.Errorf(
		"%w. cannot set register state for non-current backtraced frame",
		ErrInvalidInput)
}

func (stack *CallStack) ListInspectFrameLocalVariables() (
	[]*expression.TypedData,
	error,
) {
	frame := stack.CurrentInspectFrame()
	if frame == nil {
		return nil, fmt.Errorf("call stack frame unavailable")
	}

	entries, err := stack.LoadedElves.LocalVariableEntries(
		frame.Registers.ProgramCounter())
	if err != nil {
		return nil, err
	}

	result := []*expression.TypedData{}
	for name, entry := range entries {
		variable, err := stack.readVariable(frame, name, entry)
		if err != nil {
			return nil, err
		}

		result = append(result, variable)
	}

	sort.Slice(
		result,
		func(i int, j int) bool {
			return result[i].FormatPrefix < result[j].FormatPrefix
		})
	return result, nil
}

func (stack *CallStack) ReadInspectFrameVariableOrFunction(
	name string,
) (
	*expression.TypedData,
	error,
) {
	frame := stack.CurrentInspectFrame()
	if frame == nil {
		return nil, fmt.Errorf("call stack frame unavailable")
	}

	variable, err := stack.LoadedElves.VariableEntryWithName(
		frame.Registers.ProgramCounter(),
		name)
	if err != nil {
		return nil, err
	}
	if variable != nil {
		return stack.readVariable(frame, name, variable)
	}

	functionData, err := stack.descriptorPool.GetFunction(name)
	if err != nil {
		return nil, err
	}

	if functionData == nil {
		return nil, fmt.Errorf("%w. variable %s not found", ErrInvalidInput, name)
	}

	return functionData, err
}

func (stack *CallStack) readVariable(
	frame *CallFrame,
	name string,
	variable *dwarf.DebugInfoEntry,
) (
	*expression.TypedData,
	error,
) {
	if frame.BaseFrame != nil {
		frame = frame.BaseFrame
	}

	location, err := variable.EvaluateLocation(
		dwarf.DW_AT_location,
		frame,
		false, // in frame info
		false) // push cfa
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate location for %s: %w", name, err)
	}

	typeDie, err := variable.TypeEntry()
	if err != nil {
		return nil, fmt.Errorf("failed to get type info for %s: %w", name, err)
	}

	descriptor, err := stack.descriptorPool.GetVariableDescriptor(typeDie)
	if err != nil {
		return nil, fmt.Errorf("failed to get descriptor for %s: %w", name, err)
	}

	var address VirtualAddress
	if len(location) == 1 && location[0].Kind == dwarf.AddressLocation {
		address = VirtualAddress(location[0].Value)
	} else {
		data, err := frame.readLocationData(location, descriptor.ByteSize)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to read location data for %s: %w",
				name,
				err)
		}

		address, err = stack.InvokeMalloc(descriptor.ByteSize)
		if err != nil {
			return nil, fmt.Errorf("failed to malloc for variable: %w", err)
		}

		n, err := stack.VirtualMemory.Write(address, data)
		if err != nil {
			return nil, fmt.Errorf("failed to copy variable into memory: %w", err)
		}
		if n != len(data) {
			return nil, fmt.Errorf("failed to copy all variable data into memory")
		}
	}

	return &expression.TypedData{
		VirtualMemory:  stack.VirtualMemory,
		FormatPrefix:   name,
		DataDescriptor: descriptor,
		Address:        address,
		BitOffset:      0,
		BitSize:        8 * descriptor.ByteSize,
		Location:       location,
	}, nil
}

func (stack *CallStack) MaybeStepIntoInlinedFunction(
	status *ThreadStatus,
) (
	*ThreadStatus,
	error,
) {
	if stack.executingFrame > 0 {
		stack.executingFrame -= 1
		stack.currentInspectFrame = stack.executingFrame

		stepInStatus := newInlinedStepInStatus(status)
		err := stack.populateSourceInfo(stepInStatus)
		if err != nil {
			return nil, err
		}
		return stepInStatus, nil
	}
	return nil, nil
}

func (stack *CallStack) ExecutingFrame() *CallFrame {
	if len(stack.frames) > 0 {
		return stack.frames[stack.executingFrame]
	}
	return nil
}

func (stack *CallStack) ExecutingStack() []*CallFrame {
	if len(stack.frames) > 0 {
		return stack.frames[stack.executingFrame:]
	}
	return nil
}

func (stack *CallStack) NumUnexecutedInlinedFunctions() int {
	return stack.executingFrame
}

func (stack *CallStack) UnexecutedInlinedFunctionCodeRanges() AddressRanges {
	unexecutedFrame := stack.executingFrame - 1
	if unexecutedFrame >= 0 {
		return stack.frames[unexecutedFrame].CodeRanges
	}
	return nil
}

func (stack *CallStack) Update(
	status *ThreadStatus,
) error {
	err := stack.updateStack(status.NextInstructionAddress)
	if err != nil {
		return err
	}

	return stack.populateSourceInfo(status)
}

func (stack *CallStack) populateSourceInfo(status *ThreadStatus) error {
	if len(stack.frames) > 0 {
		executing := stack.frames[stack.executingFrame]

		status.FileEntry = executing.SourceFile
		status.Line = executing.SourceLine
	} else {
		// In case pc is not in any function, but still have line info for whatever
		// reason.
		entry, err := stack.LoadedElves.LineEntryAt(status.NextInstructionAddress)
		if err != nil {
			return err
		}
		if entry != nil {
			status.FileEntry = entry.FileEntry
			status.Line = entry.Line
		}
	}

	return nil
}

func (stack *CallStack) updateStack(
	pc VirtualAddress,
) error {
	if pc == stack.currentPC {
		return nil
	}

	currentState, err := stack.Registers.GetState()
	if err != nil {
		return err
	}

	stack.currentPC = pc
	stack.executingFrame = 0
	stack.currentInspectFrame = 0
	stack.frames = []*CallFrame{}

	for {
		hasPushed, err := stack.pushCallFrames(pc, currentState)
		if err != nil {
			return err
		}
		if !hasPushed {
			break
		}

		rules, err := stack.LoadedElves.ComputeUnwindRulesAt(pc)
		if err != nil {
			return err
		}
		if rules == nil {
			break
		}

		currentState, err = stack.unwind(
			stack.frames[len(stack.frames)-1],
			rules)
		if err != nil {
			return err
		}

		pcValue := currentState.Value(registers.ProgramCounter)
		if pcValue == nil { // undefined
			break
		}

		// NOTE: pcValue points to the return address, which is one instruction
		// after the call instruction.  Subtract one to position the pc somewhere
		// in the call instruction bytes.
		pc = VirtualAddress(pcValue.ToUint64() - 1)
	}

	for idx, frame := range stack.frames {
		if !frame.IsInlined() || frame.CodeRanges[0].Low < stack.currentPC {
			stack.executingFrame = idx
			stack.currentInspectFrame = idx
			break
		}
	}

	return nil
}

func (stack *CallStack) pushCallFrames(
	pc VirtualAddress,
	state registers.State,
) (
	bool,
	error,
) {
	// NOTE: for unwinded frames, the pc does not point to the start of an
	// instruction. Look up the line table for the start of the instruction.
	line, err := stack.LoadedElves.LineEntryAt(pc)
	if err != nil {
		return false, err
	}
	if line != nil {
		pc, err = stack.LoadedElves.LineEntryToVirtualAddress(line)
		if err != nil {
			return false, err
		}
	}

	loaded, die, err := stack.LoadedElves.
		FunctionDefinitionEntryContainingAddress(pc)
	if err != nil {
		return false, err
	}

	if die == nil { // dwarf info not available
		return false, nil
	}

	name, _, err := die.Name()
	if err != nil {
		return false, err
	}

	codeRanges, err := stack.LoadedElves.ToVirtualAddressRanges(die)
	if err != nil {
		return false, err
	}

	if !codeRanges.Contains(pc) {
		return false, fmt.Errorf("invalid function code address ranges")
	}

	baseFrame := &CallFrame{
		BaseFrame:               nil,
		File:                    loaded,
		DebugInfoEntry:          die,
		Name:                    name,
		CodeRanges:              codeRanges,
		BacktraceProgramCounter: pc,
		Registers:               state,
		memory:                  stack.VirtualMemory,
	}

	currentFrame := baseFrame
	frames := []*CallFrame{currentFrame}
	for die != nil {
		children := die.Children
		die = nil

		for _, child := range children {
			if child.Tag != dwarf.DW_TAG_inlined_subroutine {
				continue
			}

			name, _, err := child.Name()
			if err != nil {
				return false, err
			}

			codeRanges, err := stack.LoadedElves.ToVirtualAddressRanges(child)
			if err != nil {
				return false, err
			}

			if codeRanges.Contains(pc) {
				fileEntry, err := child.FileEntry()
				if err != nil {
					return false, err
				}

				line, _ := child.Line()

				currentFrame.SourceFile = fileEntry
				currentFrame.SourceLine = line

				currentFrame = &CallFrame{
					BaseFrame:               baseFrame,
					File:                    loaded,
					DebugInfoEntry:          child,
					Name:                    name,
					CodeRanges:              codeRanges,
					BacktraceProgramCounter: pc,
					Registers:               state,
					memory:                  stack.VirtualMemory,
				}
				frames = append(frames, currentFrame)

				die = child
				break
			}
		}
	}

	entry, err := stack.LoadedElves.LineEntryAt(pc)
	if err != nil {
		return false, err
	}
	if entry != nil {
		currentFrame.SourceFile = entry.FileEntry
		currentFrame.SourceLine = entry.Line
	}

	// frames is in reverse order.
	for idx := len(frames) - 1; idx >= 0; idx-- {
		stack.frames = append(stack.frames, frames[idx])
	}

	return true, nil
}

// The canonical frame address is the start of the current stack frame, and
// the register state is the values that the registers would have if the
// current function immediately returned to its caller.
func (stack *CallStack) unwind(
	currentFrame *CallFrame,
	rules *dwarf.UnwindRules,
) (
	registers.State,
	error,
) {
	previousState := currentFrame.Registers

	var cfa registers.Value
	var err error
	switch rules.CanonicalFrameAddress.Kind {
	case dwarf.CFARegisterOffsetRule:
		register, ok := registers.ById(rules.CanonicalFrameAddress.RegisterId)
		if !ok {
			return registers.State{}, fmt.Errorf(
				"register (%d) not found",
				rules.CanonicalFrameAddress.RegisterId)
		}

		value := currentFrame.Registers.Value(register)
		if value != nil {
			cfa = registers.U64(
				uint64(int64(value.ToUint64()) + rules.CanonicalFrameAddress.Offset))
		} else {
			return registers.State{}, fmt.Errorf("undefined cfa")
		}
	case dwarf.CFAExpressionRule:
		location, err := dwarf.EvaluateExpression(
			currentFrame,
			true,
			rules.CanonicalFrameAddress.ExpressionInstructions,
			false)
		if err != nil {
			return registers.State{}, fmt.Errorf(
				"cannot evaluate cfa expresion: %w",
				err)
		}

		if len(location) != 1 || location[0].Kind != dwarf.AddressLocation {
			return registers.State{}, fmt.Errorf(
				"invalid evaluated cfa location: %v",
				location)
		}

		cfa = registers.U64(location[0].Value)
	default:
		return registers.State{}, fmt.Errorf(
			"unsupported cfa rule kind (%s)",
			rules.CanonicalFrameAddress.Kind)
	}

	previousState, err = previousState.WithValue(registers.StackPointer, cfa)
	if err != nil {
		return registers.State{}, fmt.Errorf("cannot set cfa: %w", err)
	}
	currentFrame.cfa = cfa

	for registerId, rule := range rules.Registers {
		var value registers.Value

		register, ok := registers.ById(registerId)
		if !ok {
			return registers.State{}, fmt.Errorf(
				"register (%d) not found",
				registerId)
		}

		switch rule.Kind {
		case dwarf.UndefinedRule:
			// do nothing. nil value indicates undefined
		case dwarf.InRegisterRule:
			otherRegister, ok := registers.ById(rule.RegisterId)
			if !ok {
				return registers.State{}, fmt.Errorf(
					"register (%d) not found",
					rule.RegisterId)
			}
			value = currentFrame.Registers.Value(otherRegister)
		case dwarf.SameValueRule:
			value = currentFrame.Registers.Value(register)
		case dwarf.OffsetRule, dwarf.ValueOffsetRule:
			if cfa != nil {
				value = registers.U64(uint64(int64(cfa.ToUint64()) + rule.Offset))
			}
		case dwarf.ExpressionRule, dwarf.ValueExpressionRule:
			location, err := dwarf.EvaluateExpression(
				currentFrame,
				true,
				rule.ExpressionInstructions,
				true)
			if err != nil {
				return registers.State{}, err
			}

			if len(location) != 1 || location[0].Kind != dwarf.AddressLocation {
				return registers.State{}, fmt.Errorf(
					"invalid evaluated location: %v",
					location)
			}

			value = registers.U64(location[0].Value)
		default:
			return registers.State{}, fmt.Errorf(
				"unsupported register rule kind (%s)",
				rule.Kind)
		}

		if value != nil &&
			(rule.Kind == dwarf.OffsetRule || rule.Kind == dwarf.ExpressionRule) {

			if register.Size > 8 {
				return registers.State{}, fmt.Errorf("unexpected register size")
			}

			out := make([]byte, 8)
			n, err := stack.VirtualMemory.Read(VirtualAddress(value.ToUint64()), out)
			if err != nil {
				return registers.State{}, err
			}
			if n != 8 {
				panic("should never happen")
			}

			uintVal := uint64(0)
			n, err = binary.Decode(out, binary.LittleEndian, &uintVal)
			if err != nil {
				return registers.State{}, fmt.Errorf(
					"failed to decode register value: %w",
					err)
			}
			if n != 8 {
				panic("should never happen")
			}

			value = registers.U64(uintVal)
		}

		if value == nil {
			previousState = previousState.WithUndefined(register)
		} else {
			previousState, err = previousState.WithValue(register, value)
			if err != nil {
				return registers.State{}, fmt.Errorf(
					"cannot set register: %w",
					err)
			}
		}
	}

	return previousState, nil
}
