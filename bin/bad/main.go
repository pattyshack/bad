package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"strings"

	"github.com/chzyer/readline"

	"github.com/pattyshack/bad/debugger"
	. "github.com/pattyshack/bad/debugger/common"
)

func splitArg(args string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)

	first := parts[0]
	remaining := ""
	if len(parts) > 1 {
		remaining = parts[1]
	}

	return first, remaining
}

func splitAllArgs(argsStr string) []string {
	args := []string{}
	remaining := argsStr
	for len(remaining) > 0 {
		var arg string
		arg, remaining = splitArg(remaining)
		if len(arg) > 0 {
			args = append(args, arg)
		}
	}
	return args
}

type command interface {
	run(string) error
}

type namedCommand struct {
	name        string
	description string
	command
}

type subCommands []namedCommand

func (cmds subCommands) run(args string) error {
	name, remaining := splitArg(args)

	if name == "" || strings.HasPrefix("help", name) {
		cmds.printAvailableCommands()
		return nil
	}

	for _, cmd := range cmds {
		if strings.HasPrefix(cmd.name, name) {
			return cmd.run(remaining)
		}
	}

	fmt.Println("Invalid subcommand:", args)
	return nil
}

func (cmds subCommands) printAvailableCommands() {
	fmt.Println("Available subcommands:")
	for _, cmd := range cmds {
		fmt.Println("  " + cmd.name + cmd.description)
	}
}

type cmdFunc func(*debugger.Debugger, string) error

type funcCmd struct {
	debugger *debugger.Debugger
	cmdFunc
}

func newFuncCmd(debugger *debugger.Debugger, f cmdFunc) funcCmd {
	return funcCmd{
		debugger: debugger,
		cmdFunc:  f,
	}
}

func (cmd funcCmd) run(args string) error {
	return cmd.cmdFunc(cmd.debugger, args)
}

type runCmd func(string) error

func (f runCmd) run(args string) error {
	return f(args)
}

func initializeCommands(debugger *debugger.Debugger) command {
	threadCmds := subCommands{
		{
			name:        "list",
			description: "          - list all threads",
			command:     newFuncCmd(debugger, listThreads),
		},
		{
			name:        "select ",
			description: " <tid> - list all threads",
			command:     newFuncCmd(debugger, setThread),
		},
	}

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
		debugger:   debugger,
		stopPoints: debugger.BreakPoints,
	}

	watchPointCmds := stopPointCommands{
		debugger:   debugger,
		stopPoints: debugger.WatchPoints,
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

	expressionCmds := subCommands{
		{
			name:        "locals",
			description: "                - print all local variable values",
			command:     newFuncCmd(debugger, printLocalVariables),
		},
		{
			name:        "results",
			description: "               - print previously evaluated results",
			command:     newFuncCmd(debugger, printEvaluatedResults),
		},
		{
			name:        "evaluate",
			description: " <expression> - print the evaluated value",
			command:     newFuncCmd(debugger, resolveVariableExpression),
		},
		{
			name: "locate",
			description: " <name>         " +
				"- print the variable's dwarf evaluated location",
			command: newFuncCmd(debugger, printVariableLocation),
		},
	}

	return subCommands{
		{
			name: "continue",
			description: ":\n" +
				"    continue         - resume all process threads\n" +
				"    continue current - resume only the current thread",
			command: newFuncCmd(debugger, resume),
		},
		{
			name:        "next",
			description: "     - step over",
			command:     newFuncCmd(debugger, stepOver),
		},
		{
			name:        "finish",
			description: "   - step out",
			command:     newFuncCmd(debugger, stepOut),
		},
		{
			name:        "step",
			description: "     - step in",
			command:     newFuncCmd(debugger, stepIn),
		},
		{
			name:        "single",
			description: "   - single instruction step",
			command:     newFuncCmd(debugger, stepInstruction),
		},
		{
			name:        "register",
			description: " - commands for operating on registers",
			command:     registerCmds,
		},
		{
			name:        "memory",
			description: "   - commands for operating on virtual memory",
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
		{
			name: "backtrace",
			description: ":\n" +
				"    backtrace      - print backtrace\n" +
				"    backtrace up   - inspect callee frame and print backtrace\n" +
				"    backtrace down - inspect caller frame and print backtrace",
			command: newFuncCmd(debugger, backtrace),
		},
		{
			name:        "print",
			description: "       - print current thread status",
			command:     newFuncCmd(debugger, printStatus),
		},
		{
			name:        "loadedelves",
			description: " - print loaded elves",
			command:     newFuncCmd(debugger, printElves),
		},
		{
			name:        "thread",
			description: "      - commands for operating on threads",
			command:     threadCmds,
		},
		{
			name:        "expression",
			description: "  - commands for operating on global/local variables",
			command:     expressionCmds,
		},
	}
}

type noOpCmd struct{}

func (noOpCmd) run(args string) error {
	return nil
}

func resume(db *debugger.Debugger, args string) error {
	args = strings.TrimSpace(args)

	resume := db.ResumeAllUntilSignal
	if args != "" {
		if strings.HasPrefix("current", args) {
			resume = db.ResumeCurrentUntilSignal
		} else {
			fmt.Println("unexpected argument:", args)
			return nil
		}
	}

	status, err := resume()
	if err != nil {
		if errors.Is(err, ErrProcessExited) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	printThreadStatus(db, status)
	return nil
}

func stepOut(db *debugger.Debugger, args string) error {
	status, err := db.StepOut()
	if err != nil {
		if errors.Is(err, ErrProcessExited) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	printThreadStatus(db, status)
	return nil
}

func stepOver(db *debugger.Debugger, args string) error {
	status, err := db.StepOver()
	if err != nil {
		if errors.Is(err, ErrProcessExited) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	printThreadStatus(db, status)
	return nil
}

func stepIn(db *debugger.Debugger, args string) error {
	status, err := db.StepIn()
	if err != nil {
		if errors.Is(err, ErrProcessExited) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	printThreadStatus(db, status)
	return nil
}

func stepInstruction(db *debugger.Debugger, args string) error {
	status, err := db.StepInstruction()
	if err != nil {
		if errors.Is(err, ErrProcessExited) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	printThreadStatus(db, status)
	return nil
}

func listThreads(db *debugger.Debugger, args string) error {
	current, threads := db.ListThreads()
	for _, thread := range threads {
		prefix := " "
		if thread == current {
			prefix = "*"
		}

		for idx, line := range strings.Split(thread.Status().String(), "\n") {
			if idx == 0 {
				fmt.Println(prefix, line)
			} else {
				fmt.Println("   ", line)
			}
		}
		fmt.Println()
	}

	return nil
}

func setThread(db *debugger.Debugger, args string) error {
	args = strings.TrimSpace(args)

	if args == "" {
		fmt.Println("Invalid argument(s). Expected <tid>")
		return nil
	}

	tid, err := strconv.ParseInt(args, 10, 32)
	if err != nil {
		fmt.Println("Invalid tid:", err)
		return nil
	}

	err = db.SetCurrentThread(int(tid))
	if err != nil && errors.Is(err, ErrInvalidInput) {
		fmt.Println(err)
		return nil
	}

	return err
}

func printThreadLifeCycle(status *debugger.ThreadStatus) {
	if status.Running() || status.Stopped {
		fmt.Println("Thread", status.Tid, "created")
	} else if status.Exited {
		fmt.Println("Thread", status.Tid, "exited")
	} else { // Signaled (aka Terminated)
		fmt.Println("Thread", status.Tid, "terminated")
	}
}

func main() {
	pid := 0
	flag.IntVar(&pid, "p", 0, "attach to existing process pid")

	port := 0
	flag.IntVar(&port, "port", 0, "start http server (for pprof)")

	flag.Parse()
	args := flag.Args()

	if port != 0 {
		pprofServer := &http.Server{
			Addr: fmt.Sprintf(":%d", port),
		}
		go func() {
			err := pprofServer.ListenAndServe()
			if err != nil {
				panic(err)
			}
		}()
	}

	var db *debugger.Debugger
	var err error
	if pid != 0 {
		if len(args) != 0 {
			panic("unexpected arguments")
		}

		db, err = debugger.AttachTo(pid)
	} else if len(args) == 0 {
		panic("no arguments given")
	} else {
		db, err = debugger.StartCmdAndAttachTo(args[0], args[1:]...)
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

	db.WatchThreadLifeCycle(printThreadLifeCycle)

	topCmds := initializeCommands(db)

	fmt.Printf("attached to process %d\n", db.Pid)

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

		err = topCmds.run(line)
		if err != nil {
			panic(err)
		}
	}
}
