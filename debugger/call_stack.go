package debugger

import (
	"fmt"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/loadedelf"
	"github.com/pattyshack/bad/dwarf"
)

type functionFrame struct {
	die *dwarf.DebugInfoEntry

	codeRanges AddressRanges
}

type CallStack struct {
	loadedElf *loadedelf.Files

	currentPC VirtualAddress

	// On update, initialized to the inner most inlined function frame with
	// low address < pc (or the outer most non-inlined function frame).
	executingFrame int

	// The first entry is the outer most / non-inlined function. Subsequent
	// entries are nested inlined functions.  Note that the line information
	// for the current executing frame resides in the next frame (or in the line
	// table if there's no other frame).
	frames []functionFrame
}

func newCallStack(files *loadedelf.Files) *CallStack {
	return &CallStack{
		loadedElf:      files,
		currentPC:      0,
		executingFrame: 0,
	}
}

func (stack *CallStack) Update(status *ProcessStatus) error {
	if !status.Stopped {
		return nil
	}

	err := stack.updateStack(status.NextInstructionAddress)
	if err != nil {
		return err
	}

	unexecutedFrame := stack.executingFrame + 1
	if unexecutedFrame < len(stack.frames) {
		frame := stack.frames[unexecutedFrame]

		fileEntry, _ := frame.die.FileEntry()
		status.FileEntry = fileEntry

		line, _ := frame.die.Line()
		status.Line = line
	} else {
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

func (stack *CallStack) updateStack(pc VirtualAddress) error {
	if pc == stack.currentPC {
		return nil
	}

	stack.currentPC = pc
	stack.executingFrame = 0
	stack.frames = nil

	die, err := stack.loadedElf.FunctionEntryContainingAddress(pc)
	if err != nil {
		return err
	}

	if die == nil { // dwarf info not available
		return nil
	}

	codeRanges, err := stack.loadedElf.ToVirtualAddressRanges(die)
	if err != nil {
		return err
	}

	if !codeRanges.Contains(pc) {
		return fmt.Errorf("invalid function code address ranges")
	}

	prologue, err := stack.loadedElf.LineEntryAt(codeRanges[0].Low)
	if err != nil {
		return err
	}
	if prologue == nil {
		return fmt.Errorf("failed to locate function prologue line entry")
	}

	body, err := prologue.Next()
	if err != nil {
		return nil
	}
	if body == nil {
		return fmt.Errorf("failed to locate function body line entry")
	}

	bodyStart, err := stack.loadedElf.LineEntryToVirtualAddress(body)
	if err != nil {
		return err
	}

	if !codeRanges.Contains(bodyStart) {
		return fmt.Errorf("malformed dwarf DIEs")
	}

	stack.frames = []functionFrame{
		{
			die:        die,
			codeRanges: codeRanges,
		},
	}

	prevBodyStart := bodyStart
	for die != nil {
		children := die.Children
		die = nil

		for _, child := range children {
			codeRanges, err := stack.loadedElf.ToVirtualAddressRanges(child)
			if err != nil {
				return err
			}

			if codeRanges.Contains(pc) {
				// inlined function has no prologue
				bodyStart = codeRanges[0].Low

				if bodyStart < prevBodyStart {
					return fmt.Errorf("malformed dwarf DIEs")
				}

				if bodyStart < pc {
					stack.executingFrame += 1
				}

				stack.frames = append(
					stack.frames,
					functionFrame{
						die:        child,
						codeRanges: codeRanges,
					})

				die = child
				prevBodyStart = bodyStart
				break
			}
		}
	}

	return nil
}

func (stack *CallStack) MaybeStepIntoInlinedFunction() bool {
	if stack.executingFrame < len(stack.frames)-1 {
		stack.executingFrame += 1
		return true
	}
	return false
}

func (stack *CallStack) CurrentFrame() (bool, AddressRanges) {
	if stack.executingFrame < len(stack.frames) {
		isInlined := stack.executingFrame > 0
		ars := stack.frames[stack.executingFrame].codeRanges
		return isInlined, ars
	}
	return false, nil
}

func (stack *CallStack) NumUnexecutedInlinedFunctions() int {
	num := len(stack.frames) - stack.executingFrame - 1
	if num < 0 {
		num = 0
	}
	return num
}

func (stack *CallStack) UnexecutedInlinedFunctionCodeRanges() AddressRanges {
	unexecutedFrame := stack.executingFrame + 1
	if unexecutedFrame < len(stack.frames) {
		return stack.frames[unexecutedFrame].codeRanges
	}
	return nil
}
