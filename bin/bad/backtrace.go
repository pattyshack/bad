package main

import (
	"fmt"
	"strconv"

	"github.com/pattyshack/bad/debugger"
)

func backtrace(db *debugger.Debugger, args []string) error {
	printRegsAtFrame := -1 // -2 for all frames
	if len(args) > 0 {
		if args[0] == "all" {
			printRegsAtFrame = -2
		} else {
			idx, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				fmt.Println("invalid frame argument:", args[0])
				return nil
			}
			printRegsAtFrame = int(idx)
		}
	}

	matchingReg := ""
	if len(args) > 1 {
		matchingReg = args[1]
	}

	fmt.Println("Backtrace:")
	for idx, frame := range db.CallStack.ExecutingStack() {
		inlinedStr := ""
		if frame.IsInlined() {
			inlinedStr = fmt.Sprintf("(inlined in %s) ", frame.BaseFrame.Name)
		}

		fmt.Printf(
			"%4d. %s %s%s\n",
			idx,
			frame.BacktraceProgramCounter,
			inlinedStr,
			frame.Name)
		fmt.Printf("        %s:%d\n", frame.SourceFile, frame.SourceLine)

		if printRegsAtFrame == idx ||
			(!frame.IsInlined() && printRegsAtFrame == -2) {
			fmt.Println("      Registers:", matchingReg)
			printRegisters("        ", frame.Registers, matchingReg)
		}
	}

	return nil
}
