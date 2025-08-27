package bad

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/pattyshack/bad/ptrace"
)

func printWaitStatus(pid int, status syscall.WaitStatus) {
	if status.Exited() {
		fmt.Printf("process %d exited with status: %d\n", pid, status.ExitStatus())
	} else if status.Signaled() {
		fmt.Printf("process %d terminated with signal: %v\n", pid, status.Signal())
	} else if status.Stopped() {
		fmt.Printf("process %d stopped with signal: %v\n", pid, status.StopSignal())
	}
}

func Continue(db *Debugger, args []string) error {
	err := db.ResumeProcess(0)
	if err != nil {
		return err
	}

	_, err = db.WaitForProcessSignal()
	if err != nil {
		return err
	}

	return nil
}

type ProcessState string

const (
	Stopped    = ProcessState("stopped")
	Running    = ProcessState("running")
	Exited     = ProcessState("exited")
	Terminated = ProcessState("terminated")
)

type Debugger struct {
	tracer *ptrace.Tracer

	Pid         int
	ownsProcess bool

	state ProcessState
}

func AttachToProcess(pid int) (*Debugger, error) {
	tracer, err := ptrace.AttachToProcess(pid)
	if err != nil {
		return nil, err
	}

	db := &Debugger{
		tracer:      tracer,
		Pid:         tracer.Pid(),
		ownsProcess: false,
		state:       Stopped,
	}

	_, err = db.WaitForProcessSignal()
	if err != nil {
		_ = tracer.DetachFromProcess()
		return nil, err
	}

	return db, nil
}

func StartAndAttachToProcess(name string, args ...string) (*Debugger, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	tracer, err := ptrace.StartAndAttachToProcess(cmd)
	if err != nil {
		return nil, err
	}

	db := &Debugger{
		tracer:      tracer,
		Pid:         tracer.Pid(),
		ownsProcess: true,
		state:       Stopped,
	}

	_, err = db.WaitForProcessSignal()
	if err != nil {
		_ = tracer.DetachFromProcess()
		return nil, err
	}

	return db, nil
}

func (db *Debugger) ResumeProcess(signal int) error {
	err := db.tracer.ResumeProcess(signal)
	if err != nil {
		return err
	}

	db.state = Running

	fmt.Println("process", db.Pid, "running")
	return nil
}

func (db *Debugger) WaitForProcessSignal() (syscall.WaitStatus, error) {
	// NOTE: golang does not support waitpid
	var status syscall.WaitStatus
	_, err := syscall.Wait4(db.Pid, &status, 0, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to wait for process %d: %w", db.Pid, err)
	}

	if status.Exited() {
		db.state = Exited
	} else if status.Signaled() {
		db.state = Terminated
	} else if status.Stopped() {
		db.state = Stopped
	} else {
		panic(fmt.Sprintf("Unhandled wait status: %v", status))
	}

	printWaitStatus(db.Pid, status)
	return status, nil
}

func (db *Debugger) SignalToProcess(signal syscall.Signal) error {
	err := syscall.Kill(db.Pid, signal)
	if err != nil {
		return fmt.Errorf("failed to signal to process %d (%v): %w",
			db.Pid,
			signal,
			err)
	}

	fmt.Printf("signaled to process %d: %s\n", db.Pid, signal)
	return nil
}

func (db *Debugger) Close() error {
	if db.state == Running {
		err := db.SignalToProcess(syscall.SIGSTOP)
		if err != nil {
			return err
		}

		_, err = db.WaitForProcessSignal()
		if err != nil {
			return err
		}
	}

	err := db.tracer.DetachFromProcess()
	if err != nil {
		return err
	}

	err = db.SignalToProcess(syscall.SIGCONT)
	if err != nil {
		return err
	}

	if db.ownsProcess {
		err = db.SignalToProcess(syscall.SIGKILL)
		if err != nil {
			return err
		}

		_, err = db.WaitForProcessSignal()
		if err != nil {
			return err
		}
	}

	return nil
}
