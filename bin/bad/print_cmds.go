package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pattyshack/bad/debugger"
	. "github.com/pattyshack/bad/debugger/common"
)

func disassemble(db *debugger.Debugger, argsStr string) error {

	addrStr := ""
	addr := db.CurrentStatus().NextInstructionAddress

	numInstStr := ""
	numInst := 5
	for _, arg := range splitAllArgs(argsStr) {
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

func printStatus(db *debugger.Debugger, args string) error {
	printThreadStatus(db, db.CurrentStatus())
	return nil
}

func printElves(db *debugger.Debugger, args string) error {
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
