package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/chzyer/readline"

	"github.com/pattyshack/bad"
)

type command interface {
	run(*bad.Debugger, []string) error
}

type namedCommand struct {
	name        string
	description string
	command
}

type subCommands []namedCommand

func (cmds subCommands) run(db *bad.Debugger, args []string) error {
	if len(args) == 0 || strings.HasPrefix("help", args[0]) {
		cmds.printAvailableCommands()
		return nil
	}

	for _, cmd := range cmds {
		if strings.HasPrefix(cmd.name, args[0]) {
			return cmd.run(db, args[1:])
		}
	}

	fmt.Println("Invalid subcommand:", strings.Join(args, " "))
	return nil
}

func (cmds subCommands) printAvailableCommands() {
	fmt.Println("Available subcommands:")
	for _, cmd := range cmds {
		fmt.Println("  " + cmd.name + cmd.description)
	}
}

var (
	topCmds = subCommands{
		{
			name:        "register",
			description: "   - commands for operating on registers",
			command:     registerCmds,
		},
		{
			name:        "memory",
			description: "     - commands for operating on virtual memory",
			command:     memoryCmds,
		},
		{
			name:        "breakpoint",
			description: " - commands for operating on break points",
			command:     breakPointCmds,
		},
		{
			name:        "continue",
			description: "   - resume the process",
			command:     continueCmd{},
		},
		{
			name:        "step",
			description: "       - step over a single instruction",
			command:     stepInstructionCmd{},
		},
		{
			name: "disassemble",
			description: " [<n=5>] [@<addr=pc>]\n" +
				"    - disassemble <n> (default=5) instructions " +
				"at @<addr> (default=pc)",
			command: disassembleCmd{},
		},
	}

	registerCmds = subCommands{
		{
			name: "read",
			description: ":\n" +
				"    read                   - read general registers\n" +
				"    read all               - read all registers\n" +
				"    read <register>        - read the named register",
			command: readRegisterCmd{},
		},
		{
			name:        "write",
			description: " <register> <value> - write value to the named register",
			command:     writeRegisterCmd{},
		},
	}

	breakPointCmds = subCommands{
		{
			name:        "list",
			description: "              - list all break points",
			command:     listBreakPointsCmd{},
		},
		{
			name:        "set",
			description: " <address>     - create break point",
			command:     setBreakPointCmd{},
		},
		{
			name:        "remove",
			description: " <address>  - remove break point",
			command:     removeBreakPointCmd{},
		},
		{
			name:        "enable",
			description: " <address>  - enable break point",
			command:     enableBreakPointCmd{},
		},
		{
			name:        "disable",
			description: " <address> - disable break point",
			command:     disableBreakPointCmd{},
		},
	}

	memoryCmds = subCommands{
		{
			name: "read",
			description: ":\n" +
				"    read <address>                      " +
				"- read 32 bytes from address\n" +
				"    read <address> <n>                  " +
				"- read n bytes from address",
			command: readMemoryCmd{},
		},
		{
			name: "write",
			description: " <address> <byte 1> ... <byte n> " +
				"- write space separated bytes to address",
			command: writeMemoryCmd{},
		},
	}
)

type noOpCmd struct{}

func (noOpCmd) run(db *bad.Debugger, args []string) error {
	return nil
}

func printProcessState(db *bad.Debugger, state bad.ProcessState) {
	fmt.Println(state)
	if state.Stopped() {
		instructions, err := db.Disassemble(state.NextInstructionAddress, 5)
		if err != nil {
			fmt.Printf(
				"failed to disassemble instructions at %x: %s\n",
				state.NextInstructionAddress,
				err)
			return
		}

		for _, inst := range instructions {
			fmt.Println(inst)
		}
	}
}

type continueCmd struct{}

func (continueCmd) run(db *bad.Debugger, args []string) error {
	state, err := db.ResumeUntilSignal()
	if err != nil {
		if errors.Is(err, bad.ErrProcessExited) {
			fmt.Println("cannot resume. process", db.Pid, "exited")
			return nil
		}
		return err
	}

	printProcessState(db, state)
	return nil
}

type stepInstructionCmd struct{}

func (stepInstructionCmd) run(db *bad.Debugger, args []string) error {
	state, err := db.StepInstruction()
	if err != nil {
		if errors.Is(err, bad.ErrProcessExited) {
			fmt.Println("cannot step instruction. process", db.Pid, "exited")
			return nil
		}
		return err
	}

	printProcessState(db, state)
	return nil
}

type disassembleCmd struct{}

func (disassembleCmd) run(db *bad.Debugger, args []string) error {
	addrStr := ""
	addr := db.State().NextInstructionAddress

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
				addr = bad.VirtualAddress(val)
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

func main() {
	pid := 0
	flag.IntVar(&pid, "p", 0, "attach to existing process pid")

	flag.Parse()
	args := flag.Args()

	var db *bad.Debugger
	var err error
	if pid != 0 {
		if len(args) != 0 {
			panic("unexpected arguments")
		}

		db, err = bad.AttachTo(pid)
	} else if len(args) == 0 {
		panic("no arguments given")
	} else {
		db, err = bad.StartCmdAndAttachTo(args[0], args[1:]...)
	}

	if err != nil {
		panic(err)
	}

	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	fmt.Println("attached to process", db.Pid)

	rl, err := readline.New("bad > ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	lastLine := ""
	for {
		line, err := rl.Readline()
		if err != nil {
			if err == io.EOF || err == readline.ErrInterrupt {
				break
			}
			panic(err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			line = lastLine
		}
		lastLine = line

		if line == "" {
			continue
		}

		args := []string{}
		for idx, arg := range strings.Split(line, " ") {
			if arg == "" && idx != 0 {
				continue
			}
			args = append(args, arg)
		}

		if args[0] == "" {
			fmt.Println("invalid command: (empty string)")
		}

		err = topCmds.run(db, args)
		if err != nil {
			panic(err)
		}
	}
}
