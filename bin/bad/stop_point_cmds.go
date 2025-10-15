package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pattyshack/bad/debugger"
	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/stoppoint"
)

const (
	addressesBreakPoint = 1
	lineBreakPoint      = 2
	functionBreakPoint  = 3
)

type stopPointCommands struct {
	debugger   *debugger.Debugger
	stopPoints *stoppoint.StopPointSet
}

func (cmd stopPointCommands) setBreakpointSubCommands() subCommands {
	return subCommands{
		{
			name:        "function",
			description: " [-h] <name>      - set function break point",
			command: runCmd(func(args string) error {
				return cmd.setBreakPoint(functionBreakPoint, args)
			}),
		},
		{
			name:        "line",
			description: " [-h] <path> <line>   - set line break point",
			command: runCmd(func(args string) error {
				return cmd.setBreakPoint(lineBreakPoint, args)
			}),
		},
		{
			name:        "addresses",
			description: " [-h] <address>+ - set addresses break point",
			command: runCmd(func(args string) error {
				return cmd.setBreakPoint(addressesBreakPoint, args)
			}),
		},
	}
}

func (cmd stopPointCommands) SubCommands() subCommands {
	var setCmd command
	setDesc := ""
	if cmd.stopPoints.IsWatchPoints() {
		setDesc = " <address> <mode=w|rw|e> <size=1|2|4|8>\n" +
			"    - create watch point"
		setCmd = runCmd(cmd.setWatchPoint)
	} else {
		setDesc = "                      - subcommands for setting break points"
		setCmd = cmd.setBreakpointSubCommands()
	}

	return subCommands{
		{
			name: "list",
			description: fmt.Sprintf("                     - list all %ss",
				cmd.name()),
			command: runCmd(cmd.list),
		},
		{
			name:        "set",
			description: setDesc,
			command:     setCmd,
		},
		{
			name:        "remove",
			description: " <id>              - remove " + cmd.name(),
			command:     runCmd(cmd.remove),
		},
		{
			name:        "enable",
			description: " <id> [<site id>]  - enable " + cmd.name(),
			command:     runCmd(cmd.enable),
		},
		{
			name:        "disable",
			description: " <id> [<site id>] - disable " + cmd.name(),
			command:     runCmd(cmd.disable),
		},
	}

}

func (cmd stopPointCommands) name() string {
	if cmd.stopPoints.IsWatchPoints() {
		return "watch point"
	} else {
		return "break point"
	}
}

func (cmd stopPointCommands) list(args string) error {
	stopPoints := cmd.stopPoints.List()
	if len(stopPoints) == 0 {
		fmt.Println("No", cmd.name(), "set")
		return nil
	}

	fmt.Printf("Current %ss\n", cmd.name())

	for _, point := range stopPoints {
		fmt.Printf("  %d. %s (enabled = %v)\n",
			point.Id(),
			point.Type(),
			point.IsEnabled())
		fmt.Printf("     resolver: %s\n", point.Resolver())
		fmt.Println("     resolved sites:")
		for idx, site := range point.Sites() {
			fmt.Printf("       %d. %s\n", idx, site.Key())
			fmt.Printf(
				"          enabled = %v (ref count = %d)\n",
				site.IsEnabled(),
				site.RefCount())
		}
	}

	return nil
}

func (cmd stopPointCommands) parseAddressesBreakPoint(
	argsStr string,
) (
	stoppoint.StopSiteResolver,
	stoppoint.StopSiteType,
	error,
) {
	args := splitAllArgs(argsStr)

	siteType := stoppoint.NewBreakSiteType(false)
	if len(args) > 0 && args[0] == "-h" {
		siteType.IsHardware = true
		args = args[1:]
	}

	addresses := VirtualAddresses{}
	for _, arg := range args {
		address, err := cmd.debugger.LoadedElves.ParseAddress(arg)
		if err != nil {
			return nil, stoppoint.StopSiteType{}, fmt.Errorf(
				"failed to set watch point: %w",
				err)
		}

		addresses = append(addresses, address)
	}

	if len(addresses) == 0 {
		return nil, stoppoint.StopSiteType{}, fmt.Errorf(
			"failed to set break point. expected at least one address")
	}

	return cmd.debugger.NewAddressResolver(addresses...), siteType, nil
}

func (cmd stopPointCommands) parseLineBreakPoint(
	argsStr string,
) (
	stoppoint.StopSiteResolver,
	stoppoint.StopSiteType,
	error,
) {
	args := splitAllArgs(argsStr)

	siteType := stoppoint.NewBreakSiteType(false)
	if len(args) > 0 && args[0] == "-h" {
		siteType.IsHardware = true
		args = args[1:]
	}

	if len(args) != 2 {
		return nil, stoppoint.StopSiteType{}, fmt.Errorf(
			"failed to set break point. expected <path> <line>")
	}

	path := args[0]
	line, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		return nil, stoppoint.StopSiteType{}, fmt.Errorf(
			"failed to set break point. invalid line: %w", err)
	}

	return cmd.debugger.NewLineResolver(path, int(line)), siteType, nil
}

func (cmd stopPointCommands) parseFunctionBreakPoint(
	argsStr string,
) (
	stoppoint.StopSiteResolver,
	stoppoint.StopSiteType,
	error,
) {
	args := splitAllArgs(argsStr)

	siteType := stoppoint.NewBreakSiteType(false)
	if len(args) > 0 && args[0] == "-h" {
		siteType.IsHardware = true
		args = args[1:]
	}

	if len(args) != 1 {
		return nil, stoppoint.StopSiteType{}, fmt.Errorf(
			"failed to set break point. expected one function name")
	}

	return cmd.debugger.NewFunctionResolver(args[0]), siteType, nil
}

func (cmd stopPointCommands) setBreakPoint(kind int, args string) error {
	var resolver stoppoint.StopSiteResolver
	var siteType stoppoint.StopSiteType
	var err error

	switch kind {
	case addressesBreakPoint:
		resolver, siteType, err = cmd.parseAddressesBreakPoint(args)
	case lineBreakPoint:
		resolver, siteType, err = cmd.parseLineBreakPoint(args)
	case functionBreakPoint:
		resolver, siteType, err = cmd.parseFunctionBreakPoint(args)
	default:
		panic("should never happen")
	}
	if err != nil {
		fmt.Println(err)
		return nil
	}

	_, err = cmd.stopPoints.Set(resolver, siteType, true)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	return nil
}

func (cmd stopPointCommands) parseWatchPoint(
	argsStr string,
) (
	stoppoint.StopSiteResolver,
	stoppoint.StopSiteType,
	error,
) {
	args := splitAllArgs(argsStr)

	if len(args) != 3 {
		return nil, stoppoint.StopSiteType{}, fmt.Errorf(
			"failed to set watch point. expected 3 arguments: <addr> <mode> <size>")
	}
	addr, err := cmd.debugger.LoadedElves.ParseAddress(args[0])
	if err != nil {
		return nil, stoppoint.StopSiteType{}, fmt.Errorf(
			"failed to set watch point: %w",
			err)
	}

	var mode stoppoint.StopSiteMode
	switch args[1] {
	case "w":
		mode = stoppoint.WriteMode
	case "rw":
		mode = stoppoint.ReadWriteMode
	case "e":
		mode = stoppoint.ExecuteMode
	default:
		return nil, stoppoint.StopSiteType{}, fmt.Errorf(
			"failed to set watch point. invalid mode (%s). expected w|rw|e",
			args[1])
	}

	size, err := strconv.ParseInt(args[2], 0, 8)
	if err != nil {
		return nil, stoppoint.StopSiteType{}, fmt.Errorf(
			"failed to parse watch point size: %w",
			err)
	}

	resolver := cmd.debugger.NewAddressResolver(addr)
	siteType := stoppoint.NewWatchSiteType(mode, int(size))
	return resolver, siteType, nil
}

func (cmd stopPointCommands) setWatchPoint(args string) error {
	resolver, siteType, err := cmd.parseWatchPoint(args)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	_, err = cmd.stopPoints.Set(resolver, siteType, true)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	return nil
}

func (cmd stopPointCommands) remove(args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		fmt.Printf("failed to remove %s. id not specified\n", cmd.name())
		return nil
	}

	id, err := strconv.ParseInt(args, 10, 32)
	if err != nil {
		fmt.Printf("failed to parse %s id: %s\n", cmd.name(), err)
		return nil
	}

	err = cmd.stopPoints.Remove(id)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			fmt.Println(err)
			return nil
		}
		return err
	}

	return nil
}

func (cmd stopPointCommands) enable(args string) error {
	idStr, indexStr := splitArg(args)
	indexStr = strings.TrimSpace(indexStr)

	if idStr == "" {
		fmt.Printf("failed to enable %s. id not specified\n", cmd.name())
		return nil
	}

	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		fmt.Printf("failed to parse %s id: %s\n", cmd.name(), err)
		return nil
	}

	sp, ok := cmd.stopPoints.Get(id)
	if !ok {
		fmt.Printf("%s (id=%d) not found\n", cmd.name(), id)
		return nil
	}

	if indexStr == "" {
		return sp.Enable()
	}

	idx, err := strconv.ParseInt(indexStr, 10, 32)
	if err != nil {
		fmt.Printf("failed to parse %s %d site index: %s\n", cmd.name(), id, err)
		return nil
	}

	sites := sp.Sites()
	siteIdx := int(idx)
	if siteIdx < 0 || siteIdx >= len(sites) {
		fmt.Printf("%s %d site index out of bound: %d", cmd.name(), id, siteIdx)
		return nil
	}

	return sites[siteIdx].Enable()
}

func (cmd stopPointCommands) disable(args string) error {
	idStr, indexStr := splitArg(args)
	indexStr = strings.TrimSpace(indexStr)

	if idStr == "" {
		fmt.Printf("failed to disable %s. id not specified\n", cmd.name())
		return nil
	}

	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		fmt.Printf("failed to parse %s id: %s\n", cmd.name(), err)
		return nil
	}

	sp, ok := cmd.stopPoints.Get(id)
	if !ok {
		fmt.Printf("%s (id=%d) not found\n", cmd.name(), id)
		return nil
	}

	if indexStr == "" {
		return sp.Disable()
	}

	idx, err := strconv.ParseInt(indexStr, 10, 32)
	if err != nil {
		fmt.Printf("failed to parse %s %d site index: %s\n", cmd.name(), id, err)
		return nil
	}

	sites := sp.Sites()
	siteIdx := int(idx)
	if siteIdx < 0 || siteIdx >= len(sites) {
		fmt.Printf("%s %d site index out of bound: %d", cmd.name(), id, siteIdx)
		return nil
	}

	return sites[siteIdx].Disable()
}
