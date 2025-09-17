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
	run([]string) error
}

type namedCommand struct {
	name        string
	description string
	command
}

type subCommands []namedCommand

func (cmds subCommands) run(args []string) error {
	if len(args) == 0 || strings.HasPrefix("help", args[0]) {
		cmds.printAvailableCommands()
		return nil
	}

	for _, cmd := range cmds {
		if strings.HasPrefix(cmd.name, args[0]) {
			return cmd.run(args[1:])
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

type cmdFunc func(*bad.Debugger, []string) error

type funcCmd struct {
	debugger *bad.Debugger
	cmdFunc
}

func newFuncCmd(debugger *bad.Debugger, f cmdFunc) funcCmd {
	return funcCmd{
		debugger: debugger,
		cmdFunc:  f,
	}
}

func (cmd funcCmd) run(args []string) error {
	return cmd.cmdFunc(cmd.debugger, args)
}

type runCmd func([]string) error

func (f runCmd) run(args []string) error {
	return f(args)
}

func initializeCommands(debugger *bad.Debugger) command {
	registerCmds := subCommands{
		{
			name: "read",
			description: ":\n" +
				"    read                   - read general registers\n" +
				"    read all               - read all registers\n" +
				"    read <register>        - read the named register",
			command: newFuncCmd(debugger, readRegister),
		},
		{
			name:        "write",
			description: " <register> <value> - write value to the named register",
			command:     newFuncCmd(debugger, writeRegister),
		},
	}

	breakPointCmds := stopPointCommands{
		debugger:      debugger,
		stopPoints:    debugger.BreakPointSites,
		isBreakPoints: true,
	}

	watchPointCmds := stopPointCommands{
		debugger:      debugger,
		stopPoints:    debugger.WatchPoints,
		isBreakPoints: false,
	}

	memoryCmds := subCommands{
		{
			name: "read",
			description: ":\n" +
				"    read <address>                      " +
				"- read 32 bytes from address\n" +
				"    read <address> <n>                  " +
				"- read n bytes from address",
			command: newFuncCmd(debugger, readMemory),
		},
		{
			name: "write",
			description: " <address> <byte 1> ... <byte n> " +
				"- write space separated bytes to address",
			command: newFuncCmd(debugger, writeMemory),
		},
	}

	syscallCatchPolicyCmds := syscallCatchPolicyCommands{
		policy: debugger.SyscallCatchPolicy,
	}

	catchPointCmds := subCommands{
		{
			name:        "syscall",
			description: " - commands for operating on syscall catch policy",
			command:     syscallCatchPolicyCmds.SubCommands(),
		},
	}

	return subCommands{
		{
			name:        "continue",
			description: "   - resume the process",
			command:     newFuncCmd(debugger, resume),
		},
		{
			name:        "step",
			description: "       - step over a single instruction",
			command:     newFuncCmd(debugger, stepInstruction),
		},
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
			name: "disassemble",
			description: " [<n=5>] [@<addr=pc>]\n" +
				"    - disassemble <n> (default=5) instructions " +
				"at @<addr> (default=pc)",
			command: newFuncCmd(debugger, disassemble),
		},
		{
			name:        "breakpoint",
			description: " - commands for operating on break points",
			command:     breakPointCmds.SubCommands(),
		},
		{
			name:        "watchpoint",
			description: " - commands for operating on watch points",
			command:     watchPointCmds.SubCommands(),
		},
		{
			name:        "catchpoint",
			description: " - commands for operating on catch points",
			command:     catchPointCmds,
		},
	}
}

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

func resume(db *bad.Debugger, args []string) error {
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

func stepInstruction(db *bad.Debugger, args []string) error {
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

func disassemble(db *bad.Debugger, args []string) error {
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

	topCmds := initializeCommands(db)

	fmt.Printf(
		"attached to process %d (load bias: 0x%x)\n",
		db.Pid,
		db.LoadedElfFile.LoadBias)

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

		err = topCmds.run(args)
		if err != nil {
			panic(err)
		}
	}
}
