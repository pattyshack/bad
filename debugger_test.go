package bad

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"github.com/pattyshack/gt/testing/expect"
	"github.com/pattyshack/gt/testing/suite"

	"github.com/pattyshack/bad/elf"
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
	db, err := StartCmdAndAttachTo("test_targets/run_endlessly")
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
	expect.True(
		t,
		procfs.Running == status.State || procfs.WaitingForDisk == status.State)

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
	cmd := exec.Command("test_targets/run_endlessly")
	cmd.Start()
	defer cmd.Process.Kill()

	db, err := AttachTo(cmd.Process.Pid)
	expect.Nil(t, err)
	defer db.Close()

	status, err := procfs.GetProcessStatus(cmd.Process.Pid)
	expect.Nil(t, err)
	expect.Equal(t, procfs.TracingStop, status.State)

	err = db.resume()
	expect.Nil(t, err)

	status, err = procfs.GetProcessStatus(cmd.Process.Pid)
	expect.Nil(t, err)
	expect.True(
		t,
		procfs.Running == status.State || procfs.TracingStop == status.State)
}

func (DebuggerSuite) TestResumeFromStart(t *testing.T) {
	db, err := StartCmdAndAttachTo("test_targets/run_endlessly")
	expect.Nil(t, err)
	defer db.Close()

	status, err := procfs.GetProcessStatus(db.Pid)
	expect.Nil(t, err)
	expect.Equal(t, procfs.TracingStop, status.State)

	err = db.resume()
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

	err = db.resume()
	expect.Nil(t, err)

	status, err := db.waitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Exited())

	err = db.resume()
	expect.Error(t, err, "process exited")
}

func (DebuggerSuite) TestSetRegisterState(t *testing.T) {
	reader, writer, err := os.Pipe()
	expect.Nil(t, err)

	defer reader.Close()

	cmd := exec.Command("test_targets/reg_write")
	cmd.Stderr = os.Stderr
	cmd.Stdout = writer

	db, err := StartAndAttachTo(cmd)
	expect.Nil(t, err)
	defer db.Close()

	err = writer.Close()
	expect.Nil(t, err)

	err = db.resume()
	expect.Nil(t, err)

	status, err := db.waitForSignal()
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

	err = db.resume()
	expect.Nil(t, err)

	status, err = db.waitForSignal()
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

	err = db.resume()
	expect.Nil(t, err)

	status, err = db.waitForSignal()
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

	err = db.resume()
	expect.Nil(t, err)

	status, err = db.waitForSignal()
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

	err = db.resume()
	expect.Nil(t, err)

	status, err = db.waitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	n, err = reader.Read(content)
	expect.Nil(t, err)
	expect.Equal(t, "42.24", string(content[:n]))
}

func (DebuggerSuite) TestGetRegisterState(t *testing.T) {
	db, err := StartCmdAndAttachTo("test_targets/reg_read")
	expect.Nil(t, err)
	defer db.Close()

	// check r13

	err = db.resume()
	expect.Nil(t, err)

	status, err := db.waitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	regState, err := db.GetRegisterState()
	expect.Nil(t, err)

	r13, ok := db.RegisterByName("r13")
	expect.True(t, ok)

	val := regState.Value(r13)
	expect.Equal(t, 0xcafecafe, val.ToUint64())

	// check r13b

	err = db.resume()
	expect.Nil(t, err)

	status, err = db.waitForSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped())

	regState, err = db.GetRegisterState()
	expect.Nil(t, err)

	r13b, ok := db.RegisterByName("r13b")
	expect.True(t, ok)

	val = regState.Value(r13b)
	expect.Equal(t, 42, val.ToUint64())

	// check mm0

	err = db.resume()
	expect.Nil(t, err)

	status, err = db.waitForSignal()
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

	err = db.resume()
	expect.Nil(t, err)

	status, err = db.waitForSignal()
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

	err = db.resume()
	expect.Nil(t, err)

	status, err = db.waitForSignal()
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

func (DebuggerSuite) TestSoftwareBreakPointSite(t *testing.T) {
	reader, writer, err := os.Pipe()
	expect.Nil(t, err)

	defer reader.Close()

	binaryPath := "test_targets/hello_world"

	cmd := exec.Command(binaryPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = writer

	db, err := StartAndAttachTo(cmd)
	expect.Nil(t, err)
	defer db.Close()

	err = writer.Close()
	expect.Nil(t, err)

	loadAddress := db.ToVirtualAddress(elf.FileAddress(db.EntryPointAddress))

	_, err = db.BreakPointSites.Set(loadAddress, SoftwareBreakPointSiteOptions())
	expect.Nil(t, err)

	state, err := db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped())
	expect.Equal(t, syscall.SIGTRAP, state.StopSignal())

	_, pc, err := db.getProgramCounter()
	expect.Nil(t, err)
	expect.Equal(t, loadAddress, pc)

	state, err = db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Exited())
	expect.Equal(t, 0, state.ExitStatus())

	content, err := io.ReadAll(reader)
	expect.Nil(t, err)
	expect.Equal(t, "Hello world!\n", string(content))
}

func (DebuggerSuite) TestHardwareBreakPointEvadesMemoryChecksum(t *testing.T) {
	reader, writer, err := os.Pipe()
	expect.Nil(t, err)

	defer reader.Close()

	cmd := exec.Command("test_targets/anti_debugger")
	cmd.Stderr = os.Stderr
	cmd.Stdout = writer

	db, err := StartAndAttachTo(cmd)
	expect.Nil(t, err)
	defer db.Close()

	err = writer.Close()
	expect.Nil(t, err)

	state, err := db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped() && state.StopSignal() == syscall.SIGTRAP)

	buffer := make([]byte, 1024)

	n, err := reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, 8, n)

	addr := VirtualAddress(binary.LittleEndian.Uint64(buffer[:8]))

	_, err = db.BreakPointSites.Set(addr, SoftwareBreakPointSiteOptions())
	expect.Nil(t, err)

	state, err = db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped() && state.StopSignal() == syscall.SIGTRAP)

	n, err = reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, "Putting pepperoni on pizza...\n", string(buffer[:n]))

	err = db.BreakPointSites.Remove(addr)
	expect.Nil(t, err)

	_, err = db.BreakPointSites.Set(addr, HardwareBreakPointSiteOptions())
	expect.Nil(t, err)

	state, err = db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped() && state.StopSignal() == syscall.SIGTRAP)

	// verify we're at break point address
	expect.Equal(t, state.NextInstructionAddress, addr)

	state, err = db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped() && state.StopSignal() == syscall.SIGTRAP)

	n, err = reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, "Putting pineapple on pizza...\n", string(buffer[:n]))
}

func (DebuggerSuite) TestWatchPoint(t *testing.T) {
	reader, writer, err := os.Pipe()
	expect.Nil(t, err)

	defer reader.Close()

	cmd := exec.Command("test_targets/anti_debugger")
	cmd.Stderr = os.Stderr
	cmd.Stdout = writer

	db, err := StartAndAttachTo(cmd)
	expect.Nil(t, err)
	defer db.Close()

	err = writer.Close()
	expect.Nil(t, err)

	state, err := db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped() && state.StopSignal() == syscall.SIGTRAP)

	buffer := make([]byte, 1024)

	n, err := reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, 8, n)

	addr := VirtualAddress(binary.LittleEndian.Uint64(buffer[:8]))

	_, err = db.WatchPoints.Set(addr, HardwareWatchPointOptions(ReadWriteMode, 1))
	expect.Nil(t, err)

	state, err = db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped() && state.StopSignal() == syscall.SIGTRAP)

	// NOTE: force anti_debugger to read the original checksum
	state, err = db.StepInstruction()
	expect.Nil(t, err)
	expect.True(t, state.Stopped() && state.StopSignal() == syscall.SIGTRAP)

	_, err = db.BreakPointSites.Set(addr, SoftwareBreakPointSiteOptions())
	expect.Nil(t, err)

	// Hit the software breakpoint
	state, err = db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped() && state.StopSignal() == syscall.SIGTRAP)

	state, err = db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped() && state.StopSignal() == syscall.SIGTRAP)

	n, err = reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, "Putting pineapple on pizza...\n", string(buffer[:n]))
}

func (DebuggerSuite) TestReadWriteMemory(t *testing.T) {
	reader, writer, err := os.Pipe()
	expect.Nil(t, err)

	defer reader.Close()

	binaryPath := "test_targets/memory"

	cmd := exec.Command(binaryPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = writer

	db, err := StartAndAttachTo(cmd)
	expect.Nil(t, err)
	defer db.Close()

	err = writer.Close()
	expect.Nil(t, err)

	state, err := db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped() && state.StopSignal() == syscall.SIGTRAP)

	buffer := make([]byte, 1024)
	n, err := reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, 8, n)

	addr := VirtualAddress(binary.LittleEndian.Uint64(buffer[:8]))

	content := make([]byte, 8)
	count, err := db.ReadFromVirtualMemory(addr, content)
	expect.Nil(t, err)
	expect.Equal(t, 8, count)
	expect.Equal(t, 0xcafecafe, binary.LittleEndian.Uint64(content))

	state, err = db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped() && state.StopSignal() == syscall.SIGTRAP)

	n, err = reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, 8, n)

	addr = VirtualAddress(binary.LittleEndian.Uint64(buffer[:8]))

	content = []byte("hello world!\x00")
	count, err = db.WriteToVirtualMemory(addr, content)
	expect.Nil(t, err)
	expect.Equal(t, 13, count)

	state, err = db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Exited() && state.ExitStatus() == 0)

	n, err = reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, "hello world!", string(buffer[:n]))
}

func (DebuggerSuite) TestSyscallCatchpoint(t *testing.T) {
	db, err := StartCmdAndAttachTo("test_targets/anti_debugger")
	expect.Nil(t, err)
	defer db.Close()

	writeSyscall, ok := GetSyscallIdByName("write")
	expect.True(t, ok)

	db.SyscallCatchPolicy.CatchList([]SyscallId{writeSyscall})

	state, err := db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped())
	expect.Equal(t, syscall.SIGTRAP, state.StopSignal())
	expect.Equal(t, SyscallTrap, state.TrapReason)
	expect.NotNil(t, state.SyscallTrapInfo)
	expect.Equal(t, writeSyscall, state.SyscallTrapInfo.Id)
	expect.True(t, state.SyscallTrapInfo.IsEntry)

	state, err = db.ResumeUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped())
	expect.Equal(t, syscall.SIGTRAP, state.StopSignal())
	expect.Equal(t, SyscallTrap, state.TrapReason)
	expect.NotNil(t, state.SyscallTrapInfo)
	expect.Equal(t, writeSyscall, state.SyscallTrapInfo.Id)
	expect.False(t, state.SyscallTrapInfo.IsEntry)
}
