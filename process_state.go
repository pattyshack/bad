package bad

import (
	"fmt"
	"syscall"
)

type ProcessState struct {
	Pid int

	Status *syscall.WaitStatus // nil indicates running

	// Only populated when state is stopped.
	NextInstructionAddress VirtualAddress
}

func newRunningProcessState(pid int) ProcessState {
	return ProcessState{
		Pid: pid,
	}
}

func newProcessState(
	pid int,
	status syscall.WaitStatus,
) ProcessState {
	return ProcessState{
		Pid:    pid,
		Status: &status,
	}
}

func (state ProcessState) Running() bool {
	return state.Status == nil
}

func (state ProcessState) Stopped() bool {
	return state.Status != nil && state.Status.Stopped()
}

func (state ProcessState) StopSignal() syscall.Signal {
	if state.Status == nil {
		return -1
	}

	return state.Status.StopSignal()
}

func (state ProcessState) Signaled() bool {
	return state.Status != nil && state.Status.Signaled()
}

func (state ProcessState) Signal() syscall.Signal {
	if state.Status == nil {
		return -1
	}
	return state.Status.Signal()
}

func (state ProcessState) Exited() bool {
	return state.Status != nil && state.Status.Exited()
}

func (state ProcessState) ExitStatus() int {
	if state.Status == nil {
		return -1
	}

	return state.Status.ExitStatus()
}

func (state ProcessState) String() string {
	if state.Running() {
		return fmt.Sprintf("process %d running", state.Pid)
	} else if state.Stopped() {
		return fmt.Sprintf(
			"process %d stopped at %s with signal: %v\n",
			state.Pid,
			state.NextInstructionAddress,
			state.StopSignal())
	} else if state.Signaled() {
		return fmt.Sprintf(
			"process %d terminated with signal: %v",
			state.Pid,
			state.Signal())
	} else if state.Exited() {
		return fmt.Sprintf(
			"process %d exited with status: %d",
			state.Pid,
			state.ExitStatus())
	} else {
		panic("shold never happen")
	}
}
