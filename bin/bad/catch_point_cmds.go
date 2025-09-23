package main

import (
	"fmt"
	"strconv"

	"github.com/pattyshack/bad/debugger/catchpoint"
)

type syscallCatchPolicyCommands struct {
	policy *catchpoint.SyscallCatchPolicy
}

func (cmd syscallCatchPolicyCommands) SubCommands() subCommands {
	return subCommands{
		{
			name:        "current",
			description: "                     - print current syscall catch policy",
			command:     runCmd(cmd.PrintCurrent),
		},
		{
			name:        "none",
			description: "                        - don't catch any syscall",
			command:     runCmd(cmd.CatchNone),
		},
		{
			name:        "all",
			description: "                         - catch all syscalls",
			command:     runCmd(cmd.CatchAll),
		},
		{
			name:        "list",
			description: " <syscall name/number>+ - catch listed syscalls",
			command:     runCmd(cmd.CatchList),
		},
	}
}

func (cmd syscallCatchPolicyCommands) PrintCurrent(args []string) error {
	fmt.Println(cmd.policy.String())
	return nil
}

func (cmd syscallCatchPolicyCommands) CatchNone(args []string) error {
	cmd.policy.CatchNone()
	return nil
}

func (cmd syscallCatchPolicyCommands) CatchAll(args []string) error {
	cmd.policy.CatchAll()
	return nil
}

func (cmd syscallCatchPolicyCommands) CatchList(args []string) error {
	if len(args) == 0 {
		fmt.Println("no syscall name/number provided")
		return nil
	}

	ids := []catchpoint.SyscallId{}
	for _, arg := range args {
		id, ok := catchpoint.SyscallIdByName(arg)
		if ok {
			ids = append(ids, id)
			continue
		}

		num, err := strconv.ParseInt(arg, 0, 32)
		if err != nil {
			fmt.Println("invalid syscall:", arg)
			return nil
		}

		id, ok = catchpoint.SyscallIdByNumber(int(num))
		if !ok {
			fmt.Println("invalid syscall:", arg)
			return nil
		}

		ids = append(ids, id)
	}

	cmd.policy.CatchList(ids)
	return nil
}
