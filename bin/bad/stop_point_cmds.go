package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/pattyshack/bad"
)

type stopPointCommands struct {
	debugger   *bad.Debugger
	stopPoints *bad.StopPointSet

	isBreakPoints bool // false = watch point
}

func (cmd stopPointCommands) SubCommands() subCommands {
	setDesc := " <address> [-h] - create break point (-h for hardware)"
	if !cmd.isBreakPoints {
		setDesc = " <address> <mode=w|rw|e> <size=1|2|4|8>\n" +
			"    - create watch point"
	}
	return subCommands{
		{
			name:        "list",
			description: fmt.Sprintf("               - list all %ss", cmd.name()),
			command:     runCmd(cmd.list),
		},
		{
			name:        "set",
			description: setDesc,
			command:     runCmd(cmd.set),
		},
		{
			name:        "remove",
			description: " <address>   - remove " + cmd.name(),
			command:     runCmd(cmd.remove),
		},
		{
			name:        "enable",
			description: " <address>   - enable " + cmd.name(),
			command:     runCmd(cmd.enable),
		},
		{
			name:        "disable",
			description: " <address>  - disable " + cmd.name(),
			command:     runCmd(cmd.disable),
		},
	}

}

func (cmd stopPointCommands) name() string {
	if cmd.isBreakPoints {
		return "break point"
	} else {
		return "watch point"
	}
}

func (cmd stopPointCommands) list(args []string) error {
	spList := cmd.stopPoints.List()
	if len(spList) == 0 {
		fmt.Println("No", cmd.name(), "set")
		return nil
	}

	fmt.Printf("Current %ss\n", cmd.name())

	for _, sp := range spList {
		enabledStr := "false"
		if sp.IsEnabled() {
			enabledStr = "true "
		}

		if cmd.isBreakPoints {
			fmt.Printf(
				"  address = %s  enabled = %s  kind = %v\n",
				sp.Address(),
				enabledStr,
				sp.Type().Kind)
		} else {
			fmt.Printf(
				"  address = %s  enabled = %s  size = %d  mode = %s\n",
				sp.Address(),
				enabledStr,
				sp.Type().WatchSize,
				sp.Type().Mode)
		}
	}

	return nil
}

func (cmd stopPointCommands) parseBreakPoint(
	args []string,
) (
	bad.VirtualAddress,
	bad.StopPointOptions,
	error,
) {
	if len(args) < 1 {
		return 0, bad.StopPointOptions{}, fmt.Errorf(
			"failed to set break point. address not specified")
	}

	addr, err := cmd.debugger.ParseAddress(args[0])
	if err != nil {
		return 0, bad.StopPointOptions{}, fmt.Errorf(
			"failed to set break point: %w",
			err)
	}

	options := bad.SoftwareBreakPointSiteOptions()
	if len(args) > 1 && args[1] == "-h" {
		options = bad.HardwareBreakPointSiteOptions()
	}

	return addr, options, nil
}

func (cmd stopPointCommands) parseWatchPoint(
	args []string,
) (
	bad.VirtualAddress,
	bad.StopPointOptions,
	error,
) {
	if len(args) != 3 {
		return 0, bad.StopPointOptions{}, fmt.Errorf(
			"failed to set watch point. expected 3 arguments: <addr> <mode> <size>")
	}
	addr, err := cmd.debugger.ParseAddress(args[0])
	if err != nil {
		return 0, bad.StopPointOptions{}, fmt.Errorf(
			"failed to set watch point: %w",
			err)
	}

	var mode bad.StopPointMode
	switch args[1] {
	case "w":
		mode = bad.WriteMode
	case "rw":
		mode = bad.ReadWriteMode
	case "e":
		mode = bad.ExecuteMode
	default:
		return 0, bad.StopPointOptions{}, fmt.Errorf(
			"failed to set watch point. invalid mode (%s). expected w|rw|e",
			args[1])
	}

	size, err := strconv.ParseInt(args[2], 0, 8)
	if err != nil {
		return 0, bad.StopPointOptions{}, fmt.Errorf(
			"failed to parse watch point size: %w",
			err)
	}

	return addr, bad.HardwareWatchPointOptions(mode, int(size)), nil
}

func (cmd stopPointCommands) set(args []string) error {
	var addr bad.VirtualAddress
	var options bad.StopPointOptions
	var err error
	if cmd.isBreakPoints {
		addr, options, err = cmd.parseBreakPoint(args)
	} else {
		addr, options, err = cmd.parseWatchPoint(args)
	}
	if err != nil {
		fmt.Println(err)
		return nil
	}

	_, err = cmd.stopPoints.Set(addr, options)
	if err != nil {
		if errors.Is(err, bad.ErrInvalidArgument) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	return nil
}

func (cmd stopPointCommands) remove(args []string) error {
	if len(args) < 1 {
		fmt.Printf("failed to remove %s. address not specified\n", cmd.name())
		return nil
	}

	addr, err := cmd.debugger.ParseAddress(args[0])
	if err != nil {
		fmt.Printf("failed to remove %s: %s\n", cmd.name(), err)
		return nil
	}

	err = cmd.stopPoints.Remove(addr)
	if err != nil {
		if errors.Is(err, bad.ErrInvalidArgument) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	return nil
}

func (cmd stopPointCommands) enable(args []string) error {
	if len(args) < 1 {
		fmt.Printf("failed to enable %s. address not specified\n", cmd.name())
		return nil
	}

	addr, err := cmd.debugger.ParseAddress(args[0])
	if err != nil {
		fmt.Printf("failed to enable %s: %s\n", cmd.name(), err)
		return nil
	}

	sp, ok := cmd.stopPoints.Get(addr)
	if !ok {
		fmt.Printf("no %s found at %s\n", cmd.name(), addr)
		return nil
	}

	err = sp.Enable()
	if err != nil {
		return err
	}

	return nil
}

func (cmd stopPointCommands) disable(args []string) error {
	if len(args) < 1 {
		fmt.Printf("failed to disable %s. address not specified\n", cmd.name())
		return nil
	}

	addr, err := cmd.debugger.ParseAddress(args[0])
	if err != nil {
		fmt.Printf("failed to disable %s: %s\n", cmd.name(), err)
		return nil
	}

	sp, ok := cmd.stopPoints.Get(addr)
	if !ok {
		fmt.Printf("no %s found at %s\n", cmd.name(), addr)
		return nil
	}

	err = sp.Disable()
	if err != nil {
		return err
	}

	return nil
}
