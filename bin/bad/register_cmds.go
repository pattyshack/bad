package main

import (
	"fmt"
	"strings"

	"github.com/pattyshack/bad"
)

func readRegister(db *bad.Debugger, args []string) error {
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

func writeRegister(db *bad.Debugger, args []string) error {
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
