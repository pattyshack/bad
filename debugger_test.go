package bad

import (
	//  "time"
	"errors"
	"os/exec"
	"syscall"
	"testing"

	"github.com/pattyshack/gt/testing/expect"
	"github.com/pattyshack/gt/testing/suite"

	"github.com/pattyshack/bad/procfs"
)

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return !errors.Is(err, syscall.ESRCH)
}

type DebuggerSuite struct{}

func TestDebugger(t *testing.T) {
	suite.RunTests(t, &DebuggerSuite{})
}

func (DebuggerSuite) TestLaunchProcess(t *testing.T) {
	db, err := StartAndAttachToProcess("yes")
	expect.Nil(t, err)

	defer db.Close()

	expect.True(t, processExists(db.Pid))
}

func (DebuggerSuite) TestLaunchNoSuchProgram(t *testing.T) {
	db, err := StartAndAttachToProcess("invalid_program")
	expect.Nil(t, db)
	expect.Error(t, err, "failed to start process")
}

func (DebuggerSuite) TestAttachSuccess(t *testing.T) {
	cmd := exec.Command("yes")
	cmd.Start()
	defer cmd.Process.Kill()

	// sanity check
	status, err := procfs.GetProcessStatus(cmd.Process.Pid)
	expect.Nil(t, err)
	expect.Equal(t, procfs.Running, status.State)

	db, err := AttachToProcess(cmd.Process.Pid)
	expect.Nil(t, err)
	defer db.Close()

	status, err = procfs.GetProcessStatus(cmd.Process.Pid)
	expect.Nil(t, err)
	expect.Equal(t, procfs.TracingStop, status.State)
}

func (DebuggerSuite) TestAttachInvalidPid(t *testing.T) {
	_, err := AttachToProcess(0)
	expect.Error(t, err, "failed to attach to process 0")
}

/*
// XXX: flaky test
func (DebuggerSuite) TestResumeFromAttach(t *testing.T) {
	cmd := exec.Command("yes")
	cmd.Start()
	defer cmd.Process.Kill()

	db, err := AttachToProcess(cmd.Process.Pid)
	expect.Nil(t, err)
	defer db.Close()

	status, err := procfs.GetProcessStatus(cmd.Process.Pid)
	expect.Nil(t, err)
	expect.Equal(t, procfs.TracingStop, status.State)

	err = db.ResumeProcess(0)
	expect.Nil(t, err)

	status, err = procfs.GetProcessStatus(cmd.Process.Pid)
	expect.Nil(t, err)
	expect.Equal(t, procfs.Running, status.State)
}
*/

func (DebuggerSuite) TestResumeFromStart(t *testing.T) {
	db, err := StartAndAttachToProcess("yes")
	expect.Nil(t, err)
	defer db.Close()

	status, err := procfs.GetProcessStatus(db.Pid)
	expect.Nil(t, err)
	expect.Equal(t, procfs.TracingStop, status.State)

	err = db.ResumeProcess(0)
	expect.Nil(t, err)

	status, err = procfs.GetProcessStatus(db.Pid)
	expect.Nil(t, err)
	expect.Equal(t, procfs.Running, status.State)
}

func (DebuggerSuite) TestResumeAlreadyTerminated(t *testing.T) {
	db, err := StartAndAttachToProcess("echo")
	expect.Nil(t, err)
	defer db.Close()

	err = db.ResumeProcess(0)
	expect.Nil(t, err)

	status, err := db.WaitForProcessSignal()
	expect.Nil(t, err)
	expect.True(t, status.Exited())

	err = db.ResumeProcess(0)
	expect.Error(t, err, "no such process")
}
