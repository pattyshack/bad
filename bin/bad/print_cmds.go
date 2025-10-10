package main

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/pattyshack/bad/debugger"
	. "github.com/pattyshack/bad/debugger/common"
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
	for idx, frame := range db.CurrentThread().CallStack.ExecutingStack() {
		inlinedStr := ""
		if frame.IsInlined() {
			inlinedStr = fmt.Sprintf("(inlined in %s) ", frame.BaseFrame.Name)
		}

		libStr := ""
		elfFileName := ""
    if frame.SourceFile != nil {
  		elfFileName = frame.SourceFile.CompileUnit.File.FileName
    }
		if elfFileName != "" {
			libStr = fmt.Sprintf(" [%s]", path.Base(elfFileName))
		}

		fmt.Printf(
			"%4d. %s %s%s\n",
			idx,
			frame.BacktraceProgramCounter,
			inlinedStr,
			frame.Name)
		fmt.Printf("        %s:%d%s\n", frame.SourceFile, frame.SourceLine, libStr)

		if printRegsAtFrame == idx ||
			(!frame.IsInlined() && printRegsAtFrame == -2) {
			fmt.Println("      Registers:", matchingReg)
			printRegisters("        ", frame.Registers, matchingReg)
		}
	}

	return nil
}

func disassemble(db *debugger.Debugger, args []string) error {
	addrStr := ""
	addr := db.CurrentThread().Status().NextInstructionAddress

	numInstStr := ""
	numInst := 5
	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			if addrStr == "" {
				addrStr = arg
				val, err := strconv.ParseUint(arg[1:], 0, 64)
				if err != nil {
					fmt.Printf("Invalid @<addr> argument (%s): %s\n", arg, err)
					return nil
				}
				addr = VirtualAddress(val)
			} else {
				fmt.Println(
					"Invalid arguments. multiple @<addr> specified.",
					addrStr,
					"vs",
					arg)
				return nil
			}
		} else {
			if numInstStr == "" {
				numInstStr = arg
				val, err := strconv.ParseInt(arg, 0, 32)
				if err != nil {
					fmt.Printf("Invalid <n> argument (%s): %s\n", arg, err)
					return nil
				}
				numInst = int(val)
			} else {
				fmt.Println(
					"Invalid arguments. multiple <n> specified.",
					numInstStr,
					"vs",
					arg)
				return nil
			}
		}
	}

	instructions, err := db.Disassemble(addr, numInst)
	if err != nil {
		fmt.Printf(
			"failed to disassemble instructions at %x: %s\n",
			addr,
			err)
		return nil
	}

	for _, inst := range instructions {
		fmt.Println(inst)
	}

	return nil
}

func printThreadStatus(db *debugger.Debugger, status *debugger.ThreadStatus) {
	fmt.Println(status)
	if !status.Stopped {
		return
	}

	if status.FileEntry != nil {
		snippet, err := db.SourceFiles.GetSnippet(
			status.FileEntry.Path(),
			int(status.Line),
			5)
		if err != nil {
			fmt.Printf("failed to read source snippet: %s\n", err)
		} else {
			fmt.Println()
			fmt.Println(snippet)
			return
		}
	}

	instructions, err := db.Disassemble(status.NextInstructionAddress, 5)
	if err != nil {
		fmt.Printf(
			"failed to disassemble instructions at %x: %s\n",
			status.NextInstructionAddress,
			err)
		return
	}

	fmt.Println()
	for _, inst := range instructions {
		fmt.Println(inst)
	}
}

func printStatus(db *debugger.Debugger, args []string) error {
	printThreadStatus(db, db.CurrentThread().Status())
	return nil
}

func printElves(db *debugger.Debugger, args []string) error {
	fmt.Println("Loaded elves:")
	for _, file := range db.LoadedElves.Files() {
		if file.FileName == "" {
			fmt.Printf("  0x%016x: (executable)\n", file.LoadBias)
		} else {
			fmt.Printf("  0x%016x: %s\n", file.LoadBias, file.FileName)
		}
	}
	return nil
}
