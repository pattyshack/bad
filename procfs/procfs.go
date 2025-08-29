package procfs

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type ProcessState string

const (
	Running        = ProcessState("running")
	Sleeping       = ProcessState("sleeping")
	WaitingForDisk = ProcessState("waiting for disk")
	Zombie         = ProcessState("zombie")
	TracingStop    = ProcessState("tracing stop")
	Dead           = ProcessState("dead")
	Idle           = ProcessState("idle")
)

type ProcessStatus struct {
	Pid   int
	Comm  string
	State ProcessState
	Ppid  int
	Pgrp  int

	// NOTE: See man page for the full list of (52) fields.
}

func GetProcessStatus(pid int) (ProcessStatus, error) {
	contentBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return ProcessStatus{}, fmt.Errorf(
			"failed to read process %d status: %w",
			pid,
			err)
	}

	content := string(contentBytes)

	commStart := strings.Index(content, "(")
	commEnd := strings.LastIndex(content, ")")

	chunks := strings.Split(content[commEnd+2:], " ")

	pid, err = strconv.Atoi(strings.TrimSpace(content[:commStart]))
	if err != nil {
		panic("should never happen: " + err.Error())
	}

	var state ProcessState
	switch chunks[0] {
	case "R":
		state = Running
	case "S":
		state = Sleeping
	case "D":
		state = WaitingForDisk
	case "Z":
		state = Zombie
	case "t":
		state = TracingStop
	case "X":
		state = Dead
	case "I":
		state = Idle
	}

	ppid, err := strconv.Atoi(chunks[1])
	if err != nil {
		panic("should never happen: " + err.Error())
	}

	pgrp, err := strconv.Atoi(chunks[2])
	if err != nil {
		panic("should never happen: " + err.Error())
	}

	return ProcessStatus{
		Pid:   pid,
		Comm:  content[commStart+1 : commEnd],
		State: state,
		Ppid:  ppid,
		Pgrp:  pgrp,
	}, nil
}
