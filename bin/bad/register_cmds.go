package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pattyshack/bad/debugger"
	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/registers"
)

func printRegisters(
	indent string,
	state registers.State,
	match string, // "", "all", or "<name>"
) {
	if match != "" && match != "all" {
		reg, ok := registers.ByName(match)
		if !ok {
			fmt.Printf("%sInvalid register: %s", indent, match)
			return
		}

		value := state.Value(reg)
		if value == nil {
			fmt.Printf("%s%-8s (undefined)\n", indent, reg.Name)
		} else {
			fmt.Printf("%s%-8s %s\n", indent, reg.Name, value)
		}
		return
	}

	for _, reg := range registers.OrderedSpecs {
		if reg.Class == registers.GeneralClass && reg.RegisterId == -1 {
			continue
		}

		if match == "" && reg.Class != registers.GeneralClass {
			continue
		}

		name := reg.Name
		if reg.Class == registers.FloatingPointClass {
			if strings.HasPrefix(name, "st") {
				name = fmt.Sprintf("st%s/mm%s", name[2:], name[2:])
			} else if strings.HasPrefix(name, "mm") {
				continue
			}
		}

		value := state.Value(reg)
		valueStr := "(undefined)"
		if value != nil {
			valueStr = value.String()
		}

		format := "%s%-8s %s\n"
		fmt.Printf(format, indent, name, valueStr)
	}
}

func readRegister(db *debugger.Debugger, args string) error {
	state, err := db.GetInspectFrameRegisterState()
	if err != nil {
		return err
	}

	args = strings.TrimSpace(args)

	fmt.Println("Registers:", args)
	printRegisters("  ", state, args)
	return nil
}

func writeRegister(db *debugger.Debugger, argsStr string) error {
	args := splitAllArgs(argsStr)

	if len(args) != 2 {
		fmt.Println("Expected two arguments: <register> <value>")
		return nil
	}

	reg, ok := registers.ByName(args[0])
	if !ok {
		fmt.Println("Invalid register:", args[0])
		return nil
	}

	value, err := reg.ParseValue(args[1])
	if err != nil {
		fmt.Println("Invalid value:", err)
		return nil
	}

	state, err := db.GetInspectFrameRegisterState()
	if err != nil {
		return err
	}

	state, err = state.WithValue(reg, value)
	if err != nil {
		fmt.Println("Invalid value:", err)
		return nil
	}

	err = db.SetInspectFrameRegisterState(state)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	return nil
}
