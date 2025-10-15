package main

import (
	"fmt"
	"strings"

	"github.com/pattyshack/bad/debugger"
	"github.com/pattyshack/bad/debugger/registers"
	"github.com/pattyshack/bad/dwarf"
)

func printLocalVariables(db *debugger.Debugger, args string) error {
	locals, err := db.ListInspectFrameLocalVariables()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	fmt.Println("Local variables:")
	if len(locals) == 0 {
		fmt.Println("  (none)")
	}

	for idx, local := range locals {
		if idx > 0 {
			fmt.Println()
		}
		fmt.Println(local.Format("  "))
	}

	return nil
}

func printEvaluatedResults(db *debugger.Debugger, args string) error {
	fmt.Println("Evaluated results:")
	if len(db.EvaluatedResults.List()) == 0 {
		fmt.Println("  (none)")
	}
	for _, result := range db.EvaluatedResults.List() {
		if result.Index > 0 {
			fmt.Println()
		}
		fmt.Printf("  $%d: %s\n", result.Index, result.Expression)
		fmt.Println(result.Format("    "))
	}
	return nil
}

func resolveVariableExpression(db *debugger.Debugger, args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		fmt.Println("expected variable expression")
		return nil
	}

	data, err := db.ResolveVariableExpression(args)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	fmt.Printf("$%d: %s\n", data.Index, data.Expression)
	fmt.Println(data.Format("  "))
	return nil
}

func printVariableLocation(db *debugger.Debugger, args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		fmt.Println("expected variable name")
		return nil
	}

	data, err := db.ReadInspectFrameVariableOrFunction(args)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	fmt.Println("Dwarf evaluated location")
	if len(data.Location) == 0 {
		fmt.Println("  (none)")
	}

	for idx, chunk := range data.Location {
		fmt.Printf("  ")
		if len(data.Location) > 1 {
			fmt.Printf("%d. ", idx)
		}
		switch chunk.Kind {
		case dwarf.UnavailableLocation:
			fmt.Printf("(unavailable)")
		case dwarf.AddressLocation:
			fmt.Printf("address: 0x%x", chunk.Value)
		case dwarf.RegisterLocation:
			spec, ok := registers.ById(dwarf.RegisterId(chunk.Value))
			name := spec.Name
			if !ok {
				name = fmt.Sprintf("<unknown %d>", chunk.Value)
			}
			fmt.Printf("register: %s", name)
		case dwarf.ImplicitLiteralLocation, dwarf.ImplicitDataLocation:
			fmt.Printf("(implicit)")
		default:
			panic(fmt.Sprintf("unknown location kind %s", chunk.Kind))
		}

		if chunk.BitSize != 0 || chunk.BitOffset != 0 {
			fmt.Printf(" BitSize: %d BitOffset: %d\n",
				chunk.BitSize,
				chunk.BitOffset)
		} else {
			fmt.Printf("\n")
		}
	}

	return nil
}
