package bad

import (
	"errors"
	"math"
	"os"
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
	db, err := StartCmdAndAttachTo("test/targets/run_endlessly")
	expect.Nil(t, err)

	defer db.Close()

	expect.True(t, processExists(db.Pid))
}

func (DebuggerSuite) TestLaunchNoSuchProgram(t *testing.T) {
	db, err := StartCmdAndAttachTo("invalid_program")
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

	db, err := AttachTo(cmd.Process.Pid)
	expect.Nil(t, err)
	defer db.Close()

	status, err = procfs.GetProcessStatus(cmd.Process.Pid)
	expect.Nil(t, err)
	expect.Equal(t, procfs.TracingStop, status.State)
}

func (DebuggerSuite) TestAttachInvalidPid(t *testing.T) {
	_, err := AttachTo(0)
	expect.Error(t, err, "failed to attach to process 0")
}

func (DebuggerSuite) TestResumeFromAttach(t *testing.T) {
	cmd := exec.Command("test/targets/run_endlessly")
	cmd.Start()
	defer cmd.Process.Kill()

	db, err := AttachTo(cmd.Process.Pid)
	expect.Nil(t, err)
	defer db.Close()

	status, err := procfs.GetProcessStatus(cmd.Process.Pid)
	expect.Nil(t, err)
	expect.Equal(t, procfs.TracingStop, status.State)

	err = db.Resume()
	expect.Nil(t, err)

	status, err = procfs.GetProcessStatus(cmd.Process.Pid)
	expect.Nil(t, err)
	expect.True(
		t,
		procfs.Running == status.State || procfs.TracingStop == status.State)
}

func (DebuggerSuite) TestResumeFromStart(t *testing.T) {
	db, err := StartCmdAndAttachTo("test/targets/run_endlessly")
	expect.Nil(t, err)
	defer db.Close()

	status, err := procfs.GetProcessStatus(db.Pid)
	expect.Nil(t, err)
	expect.Equal(t, procfs.TracingStop, status.State)

	err = db.Resume()
	expect.Nil(t, err)

	status, err = procfs.GetProcessStatus(db.Pid)
	expect.Nil(t, err)
	expect.True(
		t,
		procfs.Running == status.State || procfs.TracingStop == status.State)
}

func (DebuggerSuite) TestResumeAlreadyTerminated(t *testing.T) {
	db, err := StartCmdAndAttachTo("echo")
	expect.Nil(t, err)
	defer db.Close()

	err = db.Resume()
	expect.Nil(t, err)

	status, err := db.WaitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Exited())

	err = db.Resume()
	expect.Error(t, err, "no such process")
}

func (DebuggerSuite) TestSetRegisterState(t *testing.T) {
	reader, writer, err := os.Pipe()
	expect.Nil(t, err)

	defer reader.Close()

	cmd := exec.Command("test/targets/reg_write")
	cmd.Stderr = os.Stderr
	cmd.Stdout = writer

	db, err := StartAndAttachTo(cmd)
	expect.Nil(t, err)
	defer db.Close()

	err = writer.Close()
	expect.Nil(t, err)

	err = db.Resume()
	expect.Nil(t, err)

	status, err := db.WaitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	// check rsi

	rsi, ok := db.RegisterByName("rsi")
	expect.True(t, ok)

	regState, err := db.GetRegisterState()
	expect.Nil(t, err)

	regState, err = regState.WithValue(rsi, Uint64Value(0xcafecafe))
	expect.Nil(t, err)

	err = db.SetRegisterState(regState)
	expect.Nil(t, err)

	err = db.Resume()
	expect.Nil(t, err)

	status, err = db.WaitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	content := make([]byte, 1024)
	n, err := reader.Read(content)
	expect.Nil(t, err)
	expect.Equal(t, "0xcafecafe", string(content[:n]))

	// check mm0

	mm0, ok := db.RegisterByName("mm0")
	expect.True(t, ok)

	regState, err = db.GetRegisterState()
	expect.Nil(t, err)

	regState, err = regState.WithValue(mm0, Uint128Value(0, 0xba5eba11))
	expect.Nil(t, err)

	err = db.SetRegisterState(regState)
	expect.Nil(t, err)

	err = db.Resume()
	expect.Nil(t, err)

	status, err = db.WaitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	n, err = reader.Read(content)
	expect.Nil(t, err)
	expect.Equal(t, "0xba5eba11", string(content[:n]))

	// check xmm0

	xmm0, ok := db.RegisterByName("xmm0")
	expect.True(t, ok)

	regState, err = db.GetRegisterState()
	expect.Nil(t, err)

	regState, err = regState.WithValue(xmm0, Float64Value(42.24))
	expect.Nil(t, err)

	err = db.SetRegisterState(regState)
	expect.Nil(t, err)

	err = db.Resume()
	expect.Nil(t, err)

	status, err = db.WaitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	n, err = reader.Read(content)
	expect.Nil(t, err)
	expect.Equal(t, "42.24", string(content[:n]))

	// check st0

	regState, err = db.GetRegisterState()
	expect.Nil(t, err)

	// NOTE: long double is not expressible in golang.
	// 42.24l 80-bit representation is:
	// 0xc3 0xf5 0x28 0x5c 0x8f 0xc2 0xf5 0xa8 0x4 0x40
	longDoubleLow := uint64(0xa8_f5_c2_8f_5c_28_f5_c3)
	longDoubleHigh := uint64(0x00_00_00_00_00_00_40_04)

	st0, ok := db.RegisterByName("st0")
	expect.True(t, ok)

	regState, err = regState.WithValue(
		st0,
		Uint128Value(longDoubleHigh, longDoubleLow))
	expect.Nil(t, err)

	// fsw' 11-13 bits track the top of the fpu stack.  Stack starts at index 0
	// (st7) and goes down instead of up, wrapping around up to 7 (st0)
	fswBits := Uint16Value(0b_00_11_10_00_00_00_00_00)

	fsw, ok := db.RegisterByName("fsw")
	expect.True(t, ok)

	regState, err = regState.WithValue(fsw, fswBits)
	expect.Nil(t, err)

	// ftw tracks which registers are valid, 2 bit per register. 0b11 means
	// empty, 0b00 means valid.
	// st0 is valid, all other st registers are empty
	ftwBits := Uint16Value(0b_00_11_11_11_11_11_11_11)

	ftw, ok := db.RegisterByName("ftw")
	expect.True(t, ok)

	regState, err = regState.WithValue(ftw, ftwBits)
	expect.Nil(t, err)

	err = db.SetRegisterState(regState)
	expect.Nil(t, err)

	err = db.Resume()
	expect.Nil(t, err)

	status, err = db.WaitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	n, err = reader.Read(content)
	expect.Nil(t, err)
	expect.Equal(t, "42.24", string(content[:n]))
}

func (DebuggerSuite) TestGetRegisterState(t *testing.T) {
	db, err := StartCmdAndAttachTo("test/targets/reg_read")
	expect.Nil(t, err)
	defer db.Close()

	// check r13

	err = db.Resume()
	expect.Nil(t, err)

	status, err := db.WaitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	regState, err := db.GetRegisterState()
	expect.Nil(t, err)

	r13, ok := db.RegisterByName("r13")
	expect.True(t, ok)

	val := regState.Value(r13)
	expect.Equal(t, 0xcafecafe, val.ToUint64())

	// check r13b

	err = db.Resume()
	expect.Nil(t, err)

	status, err = db.WaitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	regState, err = db.GetRegisterState()
	expect.Nil(t, err)

	r13b, ok := db.RegisterByName("r13b")
	expect.True(t, ok)

	val = regState.Value(r13b)
	expect.Equal(t, 42, val.ToUint64())

	// check mm0

	err = db.Resume()
	expect.Nil(t, err)

	status, err = db.WaitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	regState, err = db.GetRegisterState()
	expect.Nil(t, err)

	mm0, ok := db.RegisterByName("mm0")
	expect.True(t, ok)

	val = regState.Value(mm0)
	u128, ok := val.(Uint128)
	expect.True(t, ok)
	expect.Equal(t, 0xba5eba11, u128.Low)
	// NOTE: the high bits contains garbage

	// check xmm0

	err = db.Resume()
	expect.Nil(t, err)

	status, err = db.WaitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	regState, err = db.GetRegisterState()
	expect.Nil(t, err)

	xmm0, ok := db.RegisterByName("xmm0")
	expect.True(t, ok)

	val = regState.Value(xmm0)
	u128, ok = val.(Uint128)
	expect.True(t, ok)
	expect.Equal(t, math.Float64bits(64.125), u128.Low)
	expect.Equal(t, 0, u128.High)

	// check st0

	err = db.Resume()
	expect.Nil(t, err)

	status, err = db.WaitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	regState, err = db.GetRegisterState()
	expect.Nil(t, err)

	st0, ok := db.RegisterByName("st0")
	expect.True(t, ok)

	val = regState.Value(st0)
	u128, ok = val.(Uint128)
	expect.True(t, ok)

	// NOTE: long double is not expressible in golang.
	// 64.125 80-bit representation is:
	// 0 0 0 0 0 0 0x40 0x80 0x5 0x40
	expect.Equal(t, 0x80_40_00_00_00_00_00_00, u128.Low)
	expect.Equal(t, 0x40_05, u128.High)
}
