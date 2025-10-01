package debugger

import (
	"encoding/binary"
	"fmt"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/loadedelf"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/debugger/registers"
	"github.com/pattyshack/bad/dwarf"
)

type CallFrame struct {
	// Inlined frame's base frame.
	BaseFrame *CallFrame

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
}

func (frame *CallFrame) IsInlined() bool {
	return frame.BaseFrame != nil
}

type CallStack struct {
	loadedElf *loadedelf.Files

	memory *memory.VirtualMemory

	currentPC VirtualAddress

	// On update, initialized to the inner most inlined function frame with
	// low address < pc (or the outer most non-inlined function frame).
	executingFrame int

	// The first entry is the top of the call stack.
	frames []*CallFrame
}

func newCallStack(files *loadedelf.Files, vm *memory.VirtualMemory) *CallStack {
	return &CallStack{
		loadedElf:      files,
		memory:         vm,
		currentPC:      0,
		executingFrame: 0,
	}
}

func (stack *CallStack) MaybeStepIntoInlinedFunction(
	status *ProcessStatus,
) (
	*ProcessStatus,
	error,
) {
	if stack.executingFrame > 0 {
		stack.executingFrame -= 1

		stepInStatus := newInlinedStepInStatus(status)
		err := stack.populateSourceInfo(stepInStatus)
		if err != nil {
			return nil, err
		}
		return stepInStatus, nil
	}
	return nil, nil
}

func (stack *CallStack) CurrentFrame() *CallFrame {
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
	status *ProcessStatus,
	currentState registers.State,
) error {
	err := stack.updateStack(status.NextInstructionAddress, currentState)
	if err != nil {
		return err
	}

	return stack.populateSourceInfo(status)
}

func (stack *CallStack) populateSourceInfo(status *ProcessStatus) error {
	if len(stack.frames) > 0 {
		executing := stack.frames[stack.executingFrame]

		status.FileEntry = executing.SourceFile
		status.Line = executing.SourceLine
	} else {
		// In case pc is not in any function, but still have line info for whatever
		// reason.
		entry, err := stack.loadedElf.LineEntryAt(status.NextInstructionAddress)
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
	currentState registers.State,
) error {
	if pc == stack.currentPC {
		return nil
	}

	stack.currentPC = pc
	stack.executingFrame = 0
	stack.frames = []*CallFrame{}

	for {
		hasPushed, err := stack.pushCallFrames(pc, currentState)
		if err != nil {
			return err
		}
		if !hasPushed {
			break
		}

		rules, err := stack.loadedElf.ComputeUnwindRulesAt(pc)
		if err != nil {
			return err
		}
		if rules == nil {
			break
		}

		_, currentState, err = stack.unwind(currentState, rules)
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
	line, err := stack.loadedElf.LineEntryAt(pc)
	if err != nil {
		return false, err
	}
	if line != nil {
		pc, err = stack.loadedElf.LineEntryToVirtualAddress(line)
		if err != nil {
			return false, err
		}
	}

	die, err := stack.loadedElf.FunctionEntryContainingAddress(pc)
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

	codeRanges, err := stack.loadedElf.ToVirtualAddressRanges(die)
	if err != nil {
		return false, err
	}

	if !codeRanges.Contains(pc) {
		return false, fmt.Errorf("invalid function code address ranges")
	}

	baseFrame := &CallFrame{
		BaseFrame:               nil,
		DebugInfoEntry:          die,
		Name:                    name,
		CodeRanges:              codeRanges,
		BacktraceProgramCounter: pc,
		Registers:               state,
	}

	currentFrame := baseFrame
	frames := []*CallFrame{currentFrame}
	for die != nil {
		children := die.Children
		die = nil

		for _, child := range children {
			name, _, err := child.Name()
			if err != nil {
				return false, err
			}

			codeRanges, err := stack.loadedElf.ToVirtualAddressRanges(child)
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
					DebugInfoEntry:          child,
					Name:                    name,
					CodeRanges:              codeRanges,
					BacktraceProgramCounter: pc,
					Registers:               state,
				}
				frames = append(frames, currentFrame)

				die = child
				break
			}
		}
	}

	entry, err := stack.loadedElf.LineEntryAt(pc)
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
	currentState registers.State,
	rules *dwarf.UnwindRules,
) (
	registers.Value, // Canonical frame address
	registers.State,
	error,
) {
	previousState := currentState

	var cfa registers.Value
	var err error
	switch rules.CanonicalFrameAddress.Kind {
	case dwarf.CFARegisterOffsetRule:
		register, ok := registers.ById(rules.CanonicalFrameAddress.RegisterId)
		if !ok {
			return nil, registers.State{}, fmt.Errorf(
				"register (%d) not found",
				rules.CanonicalFrameAddress.RegisterId)
		}

		value := currentState.Value(register)
		if value != nil {
			cfa = registers.U64(
				uint64(int64(value.ToUint64()) + rules.CanonicalFrameAddress.Offset))

			previousState, err = previousState.WithValue(
				registers.StackPointer,
				cfa)
			if err != nil {
				return nil, registers.State{}, fmt.Errorf("cannot set cfa: %w", err)
			}
		} else {
			previousState = previousState.WithUndefined(registers.StackPointer)
		}
	default:
		return nil, registers.State{}, fmt.Errorf(
			"unsupported cfa rule kind (%s)",
			rules.CanonicalFrameAddress.Kind)
	}

	for registerId, rule := range rules.Registers {
		var value registers.Value

		register, ok := registers.ById(registerId)
		if !ok {
			return nil, registers.State{}, fmt.Errorf(
				"register (%d) not found",
				registerId)
		}

		switch rule.Kind {
		case dwarf.UndefinedRule:
			// do nothing. nil value indicates undefined
		case dwarf.InRegisterRule:
			otherRegister, ok := registers.ById(rule.RegisterId)
			if !ok {
				return nil, registers.State{}, fmt.Errorf(
					"register (%d) not found",
					rule.RegisterId)
			}
			value = currentState.Value(otherRegister)
		case dwarf.SameValueRule:
			value = currentState.Value(register)
		case dwarf.OffsetRule, dwarf.ValueOffsetRule:
			if cfa != nil {
				value = registers.U64(uint64(int64(cfa.ToUint64()) + rule.Offset))
			}
		case dwarf.ExpressionRule:
			panic("TODO")
		case dwarf.ValueExpressionRule:
			panic("TODO")
		default:
			return nil, registers.State{}, fmt.Errorf(
				"unsupported register rule kind (%s)",
				rule.Kind)
		}

		if value != nil &&
			(rule.Kind == dwarf.OffsetRule || rule.Kind == dwarf.ExpressionRule) {

			if register.Size > 8 {
				return nil, registers.State{}, fmt.Errorf("unexpected register size")
			}

			out := make([]byte, 8)
			n, err := stack.memory.Read(VirtualAddress(value.ToUint64()), out)
			if err != nil {
				return nil, registers.State{}, err
			}
			if n != 8 {
				panic("should never happen")
			}

			uintVal := uint64(0)
			n, err = binary.Decode(out, binary.LittleEndian, &uintVal)
			if err != nil {
				return nil, registers.State{}, fmt.Errorf(
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
				return nil, registers.State{}, fmt.Errorf(
					"cannot set register: %w",
					err)
			}
		}
	}

	return cfa, previousState, nil
}
