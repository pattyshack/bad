package main

import (
	"errors"
	"fmt"

	"github.com/pattyshack/bad"
)

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
