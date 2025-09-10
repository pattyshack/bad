package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
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
			description: " - step over a single instruction",
			command:     stepInstructionCmd{},
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
)

type noOpCmd struct{}

func (noOpCmd) run(db *bad.Debugger, args []string) error {
	return nil
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

	fmt.Println(state)
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

	fmt.Println(state)
	return nil
}

type readRegisterCmd struct{}

func (readRegisterCmd) run(db *bad.Debugger, args []string) error {
	if len(args) > 0 && args[0] != "all" {
		reg, ok := db.RegisterByName(args[0])
		if !ok {
			fmt.Println("Invalid register:", args[0])
			return nil
		}

		state, err := db.GetRegisterState()
		if err != nil {
			return err
		}

		fmt.Printf("%s: %s\n", reg.Name, state.Value(reg))
		return nil
	}

	state, err := db.GetRegisterState()
	if err != nil {
		return err
	}

	for _, reg := range db.Registers {
		// Skip printing general sub registers
		if reg.Class == bad.GeneralRegister && reg.DwarfId == -1 {
			continue
		}

		if len(args) == 0 && reg.Class != bad.GeneralRegister {
			continue
		}

		name := reg.Name
		if reg.Class == bad.FloatingPointRegister {
			if strings.HasPrefix(name, "st") {
				name = fmt.Sprintf("st%s/mm%s", name[2:], name[2:])
			} else if strings.HasPrefix(name, "mm") {
				continue
			}
		}

		format := "%s:\t\t%s\n"
		if len(name) >= 7 {
			format = "%s:\t%s\n"
		}
		fmt.Printf(format, name, state.Value(reg))
	}

	return nil
}

type writeRegisterCmd struct{}

func (writeRegisterCmd) run(db *bad.Debugger, args []string) error {
	if len(args) != 2 {
		fmt.Println("Expected two arguments: <register> <value>")
		return nil
	}

	reg, ok := db.RegisterByName(args[0])
	if !ok {
		fmt.Println("Invalid register:", args[0])
		return nil
	}

	value, err := reg.ParseValue(args[1])
	if err != nil {
		fmt.Println("Invalid value:", err)
		return nil
	}

	state, err := db.GetRegisterState()
	if err != nil {
		return err
	}

	state, err = state.WithValue(reg, value)
	if err != nil {
		fmt.Println("Invalid value:", err)
		return nil
	}

	return db.SetRegisterState(state)
}

type listBreakPointsCmd struct{}

func (listBreakPointsCmd) run(db *bad.Debugger, args []string) error {
	sites := db.BreakPointSites.List()
	if len(sites) == 0 {
		fmt.Println("No break points set")
		return nil
	}

	fmt.Println("Current break points")
	for _, site := range sites {
		fmt.Println("  address =", site.Address(), " enabled =", site.IsEnabled())
	}

	return nil
}

type setBreakPointCmd struct{}

func (setBreakPointCmd) run(db *bad.Debugger, args []string) error {
	if len(args) < 1 {
		fmt.Println("failed to set break point. address not specified")
		return nil
	}

	addr, err := bad.ParseVirtualAddress(args[0])
	if err != nil {
		fmt.Println("failed to set break point:", err)
		return nil
	}

	_, err = db.BreakPointSites.Set(addr)
	if err != nil {
		if errors.Is(err, bad.ErrInvalidStopPointAddress) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	return nil
}

type removeBreakPointCmd struct{}

func (removeBreakPointCmd) run(db *bad.Debugger, args []string) error {
	if len(args) < 1 {
		fmt.Println("failed to remove break point. address not specified")
		return nil
	}

	addr, err := bad.ParseVirtualAddress(args[0])
	if err != nil {
		fmt.Println("failed to remove break point:", err)
		return nil
	}

	err = db.BreakPointSites.Remove(addr)
	if err != nil {
		if errors.Is(err, bad.ErrInvalidStopPointAddress) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	return nil
}

type enableBreakPointCmd struct{}

func (enableBreakPointCmd) run(db *bad.Debugger, args []string) error {
	if len(args) < 1 {
		fmt.Println("failed to enable break point. address not specified")
		return nil
	}

	addr, err := bad.ParseVirtualAddress(args[0])
	if err != nil {
		fmt.Println("failed to enable break point:", err)
		return nil
	}

	site, ok := db.BreakPointSites.Get(addr)
	if !ok {
		fmt.Println("no break point found at", addr)
		return nil
	}

	err = site.Enable()
	if err != nil {
		return err
	}

	return nil
}

type disableBreakPointCmd struct{}

func (disableBreakPointCmd) run(db *bad.Debugger, args []string) error {
	if len(args) < 1 {
		fmt.Println("failed to disable break point. address not specified")
		return nil
	}

	addr, err := bad.ParseVirtualAddress(args[0])
	if err != nil {
		fmt.Println("failed to disable break point:", err)
		return nil
	}

	site, ok := db.BreakPointSites.Get(addr)
	if !ok {
		fmt.Println("no break point found at", addr)
		return nil
	}

	err = site.Disable()
	if err != nil {
		return err
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

		args := strings.Split(line, " ")
		if args[0] == "" {
			fmt.Println("invalid command: (empty string)")
		}

		err = topCmds.run(db, args)
		if err != nil {
			panic(err)
		}
	}
}
