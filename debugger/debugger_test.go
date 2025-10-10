package debugger

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path"
	"syscall"
	"testing"

	"github.com/pattyshack/gt/testing/expect"
	"github.com/pattyshack/gt/testing/suite"

	"github.com/pattyshack/bad/debugger/catchpoint"
	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/registers"
	"github.com/pattyshack/bad/debugger/stoppoint"
	"github.com/pattyshack/bad/dwarf"
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

	err = db.CurrentThread().threadTracer.Resume(0)
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

	err = db.CurrentThread().threadTracer.Resume(0)
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

	status, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Exited)

	_, err = db.ResumeAllUntilSignal()
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

	status, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)

	// check rsi

	rsi, ok := registers.ByName("rsi")
	expect.True(t, ok)

	regState, err := db.CurrentThread().Registers.GetState()
	expect.Nil(t, err)

	regState, err = regState.WithValue(rsi, registers.U64(0xcafecafe))
	expect.Nil(t, err)

	err = db.CurrentThread().Registers.SetState(regState)
	expect.Nil(t, err)

	status, err = db.ResumeCurrentUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)

	content := make([]byte, 1024)
	n, err := reader.Read(content)
	expect.Nil(t, err)
	expect.Equal(t, "0xcafecafe", string(content[:n]))

	// check mm0

	mm0, ok := registers.ByName("mm0")
	expect.True(t, ok)

	regState, err = db.CurrentThread().Registers.GetState()
	expect.Nil(t, err)

	regState, err = regState.WithValue(mm0, registers.U128(0, 0xba5eba11))
	expect.Nil(t, err)

	err = db.CurrentThread().Registers.SetState(regState)
	expect.Nil(t, err)

	status, err = db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)

	n, err = reader.Read(content)
	expect.Nil(t, err)
	expect.Equal(t, "0xba5eba11", string(content[:n]))

	// check xmm0

	xmm0, ok := registers.ByName("xmm0")
	expect.True(t, ok)

	regState, err = db.CurrentThread().Registers.GetState()
	expect.Nil(t, err)

	regState, err = regState.WithValue(xmm0, registers.F64(42.24))
	expect.Nil(t, err)

	err = db.CurrentThread().Registers.SetState(regState)
	expect.Nil(t, err)

	status, err = db.ResumeCurrentUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)

	n, err = reader.Read(content)
	expect.Nil(t, err)
	expect.Equal(t, "42.24", string(content[:n]))

	// check st0

	regState, err = db.CurrentThread().Registers.GetState()
	expect.Nil(t, err)

	// NOTE: long double is not expressible in golang.
	// 42.24l 80-bit representation is:
	// 0xc3 0xf5 0x28 0x5c 0x8f 0xc2 0xf5 0xa8 0x4 0x40
	longDoubleLow := uint64(0xa8_f5_c2_8f_5c_28_f5_c3)
	longDoubleHigh := uint64(0x00_00_00_00_00_00_40_04)

	st0, ok := registers.ByName("st0")
	expect.True(t, ok)

	regState, err = regState.WithValue(
		st0,
		registers.U128(longDoubleHigh, longDoubleLow))
	expect.Nil(t, err)

	// fsw' 11-13 bits track the top of the fpu stack.  Stack starts at index 0
	// (st7) and goes down instead of up, wrapping around up to 7 (st0)
	fswBits := registers.U16(0b_00_11_10_00_00_00_00_00)

	fsw, ok := registers.ByName("fsw")
	expect.True(t, ok)

	regState, err = regState.WithValue(fsw, fswBits)
	expect.Nil(t, err)

	// ftw tracks which registers are valid, 2 bit per register. 0b11 means
	// empty, 0b00 means valid.
	// st0 is valid, all other st registers are empty
	ftwBits := registers.U16(0b_00_11_11_11_11_11_11_11)

	ftw, ok := registers.ByName("ftw")
	expect.True(t, ok)

	regState, err = regState.WithValue(ftw, ftwBits)
	expect.Nil(t, err)

	err = db.CurrentThread().Registers.SetState(regState)
	expect.Nil(t, err)

	status, err = db.ResumeAllUntilSignal()
	expect.True(t, status.Stopped)

	n, err = reader.Read(content)
	expect.Nil(t, err)
	expect.Equal(t, "42.24", string(content[:n]))
}

func (DebuggerSuite) TestGetRegisterState(t *testing.T) {
	db, err := StartCmdAndAttachTo("test_targets/reg_read")
	expect.Nil(t, err)
	defer db.Close()

	// check r13

	status, err := db.ResumeCurrentUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)

	regState, err := db.CurrentThread().Registers.GetState()
	expect.Nil(t, err)
	r13, ok := registers.ByName("r13")
	expect.True(t, ok)

	val := regState.Value(r13)
	expect.Equal(t, 0xcafecafe, val.ToUint64())

	// check r13b

	status, err = db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)

	regState, err = db.CurrentThread().Registers.GetState()
	expect.Nil(t, err)

	r13b, ok := registers.ByName("r13b")
	expect.True(t, ok)

	val = regState.Value(r13b)
	expect.Equal(t, 42, val.ToUint64())

	// check mm0

	status, err = db.ResumeCurrentUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)

	regState, err = db.CurrentThread().Registers.GetState()
	expect.Nil(t, err)

	mm0, ok := registers.ByName("mm0")
	expect.True(t, ok)

	val = regState.Value(mm0)
	u128, ok := val.(registers.Uint128)
	expect.True(t, ok)
	expect.Equal(t, 0xba5eba11, u128.Low)
	// NOTE: the high bits contains garbage

	// check xmm0

	status, err = db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)

	regState, err = db.CurrentThread().Registers.GetState()
	expect.Nil(t, err)

	xmm0, ok := registers.ByName("xmm0")
	expect.True(t, ok)

	val = regState.Value(xmm0)
	u128, ok = val.(registers.Uint128)
	expect.True(t, ok)
	expect.Equal(t, math.Float64bits(64.125), u128.Low)
	expect.Equal(t, 0, u128.High)

	// check st0

	status, err = db.ResumeCurrentUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)

	regState, err = db.CurrentThread().Registers.GetState()
	expect.Nil(t, err)

	st0, ok := registers.ByName("st0")
	expect.True(t, ok)

	val = regState.Value(st0)
	u128, ok = val.(registers.Uint128)
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

	loadAddress := db.LoadedElves.EntryPoint()

	_, err = db.BreakPoints.Set(
		db.NewAddressResolver(loadAddress),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	state, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped)
	expect.Equal(t, syscall.SIGTRAP, state.StopSignal)

	_, pc, err := db.CurrentThread().Registers.GetProgramCounter()
	expect.Nil(t, err)
	expect.Equal(t, loadAddress, pc)

	state, err = db.ResumeCurrentUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Exited)
	expect.Equal(t, 0, state.ExitStatus)

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

	state, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped && state.StopSignal == syscall.SIGTRAP)

	buffer := make([]byte, 1024)

	n, err := reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, 8, n)

	addr := VirtualAddress(binary.LittleEndian.Uint64(buffer[:8]))

	breakPoint, err := db.BreakPoints.Set(
		db.NewAddressResolver(addr),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	state, err = db.ResumeCurrentUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped && state.StopSignal == syscall.SIGTRAP)

	n, err = reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, "Putting pepperoni on pizza...\n", string(buffer[:n]))

	err = db.BreakPoints.Remove(breakPoint.Id())
	expect.Nil(t, err)

	_, err = db.BreakPoints.Set(
		db.NewAddressResolver(addr),
		stoppoint.NewBreakSiteType(true),
		true)
	expect.Nil(t, err)

	state, err = db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped && state.StopSignal == syscall.SIGTRAP)

	// verify we're at break point address
	expect.Equal(t, state.NextInstructionAddress, addr)

	state, err = db.ResumeCurrentUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped && state.StopSignal == syscall.SIGTRAP)

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

	state, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped && state.StopSignal == syscall.SIGTRAP)

	buffer := make([]byte, 1024)

	n, err := reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, 8, n)

	addr := VirtualAddress(binary.LittleEndian.Uint64(buffer[:8]))

	_, err = db.WatchPoints.Set(
		db.NewAddressResolver(addr),
		stoppoint.NewWatchSiteType(stoppoint.ReadWriteMode, 1),
		true)
	expect.Nil(t, err)

	state, err = db.ResumeCurrentUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped && state.StopSignal == syscall.SIGTRAP)

	// NOTE: force anti_debugger to read the original checksum
	state, err = db.StepInstruction()
	expect.Nil(t, err)
	expect.True(t, state.Stopped && state.StopSignal == syscall.SIGTRAP)

	_, err = db.BreakPoints.Set(
		db.NewAddressResolver(addr),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	// Hit the software breakpoint
	state, err = db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped && state.StopSignal == syscall.SIGTRAP)

	state, err = db.ResumeCurrentUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped && state.StopSignal == syscall.SIGTRAP)

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

	state, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped && state.StopSignal == syscall.SIGTRAP)

	buffer := make([]byte, 1024)
	n, err := reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, 8, n)

	addr := VirtualAddress(binary.LittleEndian.Uint64(buffer[:8]))

	content := make([]byte, 8)
	count, err := db.VirtualMemory.Read(addr, content)
	expect.Nil(t, err)
	expect.Equal(t, 8, count)
	expect.Equal(t, 0xcafecafe, binary.LittleEndian.Uint64(content))

	state, err = db.ResumeCurrentUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped && state.StopSignal == syscall.SIGTRAP)

	n, err = reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, 8, n)

	addr = VirtualAddress(binary.LittleEndian.Uint64(buffer[:8]))

	content = []byte("hello world!\x00")
	count, err = db.VirtualMemory.Write(addr, content)
	expect.Nil(t, err)
	expect.Equal(t, 13, count)

	state, err = db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Exited && state.ExitStatus == 0)

	n, err = reader.Read(buffer)
	expect.Nil(t, err)
	expect.Equal(t, "hello world!", string(buffer[:n]))
}

func (DebuggerSuite) TestSyscallCatchpoint(t *testing.T) {
	db, err := StartCmdAndAttachTo("test_targets/anti_debugger")
	expect.Nil(t, err)
	defer db.Close()

	writeSyscall, ok := catchpoint.SyscallIdByName("write")
	expect.True(t, ok)

	db.SyscallCatchPolicy.CatchList([]catchpoint.SyscallId{writeSyscall})

	state, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped)
	expect.Equal(t, syscall.SIGTRAP, state.StopSignal)
	expect.Equal(t, SyscallTrap, state.TrapKind)
	expect.NotNil(t, state.SyscallTrapInfo)
	expect.Equal(t, writeSyscall, state.SyscallTrapInfo.Id)
	expect.True(t, state.SyscallTrapInfo.IsEntry)

	state, err = db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, state.Stopped)
	expect.Equal(t, syscall.SIGTRAP, state.StopSignal)
	expect.Equal(t, SyscallTrap, state.TrapKind)
	expect.NotNil(t, state.SyscallTrapInfo)
	expect.Equal(t, writeSyscall, state.SyscallTrapInfo.Id)
	expect.False(t, state.SyscallTrapInfo.IsEntry)
}

func (DebuggerSuite) TestSourceLevelBreakPoints(t *testing.T) {
	cmd := exec.Command("test_targets/overloaded")
	db, err := StartAndAttachTo(cmd)
	expect.Nil(t, err)
	defer db.Close()

	_, err = db.BreakPoints.Set(
		db.NewLineResolver("overloaded.cpp", 17),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	status, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SoftwareTrap, status.TrapKind)
	expect.Equal(t, "overloaded.cpp", status.FileEntry.Name)
	expect.Equal(t, 17, status.Line)

	point, err := db.BreakPoints.Set(
		db.NewFunctionResolver("print_type"),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	sites := point.Sites()
	expect.Equal(t, 3, len(sites))

	err = sites[0].Disable()
	expect.Nil(t, err)

	status, err = db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SoftwareTrap, status.TrapKind)
	expect.Equal(t, "overloaded.cpp", status.FileEntry.Name)
	expect.Equal(t, 9, status.Line)

	status, err = db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SoftwareTrap, status.TrapKind)
	expect.Equal(t, "overloaded.cpp", status.FileEntry.Name)
	expect.Equal(t, 13, status.Line)

	status, err = db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Exited)
}

func (DebuggerSuite) TestSourceLevelStepping(t *testing.T) {
	cmd := exec.Command("test_targets/step")
	db, err := StartAndAttachTo(cmd)
	expect.Nil(t, err)
	defer db.Close()

	_, err = db.BreakPoints.Set(
		db.NewFunctionResolver("main"),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	status, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SoftwareTrap, status.TrapKind)
	expect.Equal(t, "main", status.FunctionName)

	oldPC := status.NextInstructionAddress

	status, err = db.StepOver()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SingleStepTrap, status.TrapKind)
	expect.Equal(t, "main", status.FunctionName)
	expect.NotEqual(t, oldPC, status.NextInstructionAddress)

	status, err = db.StepIn()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SingleStepTrap, status.TrapKind)
	expect.Equal(t, "find_happiness", status.FunctionName)
	expect.Equal(
		t,
		2,
		db.CurrentThread().CallStack.NumUnexecutedInlinedFunctions())

	oldPC = status.NextInstructionAddress

	status, err = db.StepIn()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SingleStepTrap, status.TrapKind)
	expect.Equal(t, "find_happiness", status.FunctionName)
	expect.Equal(
		t,
		1,
		db.CurrentThread().CallStack.NumUnexecutedInlinedFunctions())
	expect.Equal(t, oldPC, status.NextInstructionAddress)

	status, err = db.StepOut()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SingleStepTrap, status.TrapKind)
	expect.Equal(t, "find_happiness", status.FunctionName)
	expect.NotEqual(t, oldPC, status.NextInstructionAddress)

	status, err = db.StepOut()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SingleStepTrap, status.TrapKind)
	expect.Equal(t, "main", status.FunctionName)
}

func (DebuggerSuite) TestStackUnwinding(t *testing.T) {
	cmd := exec.Command("test_targets/step")
	db, err := StartAndAttachTo(cmd)
	expect.Nil(t, err)
	defer db.Close()

	_, err = db.BreakPoints.Set(
		db.NewFunctionResolver("scratch_ears"),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	status, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SoftwareTrap, status.TrapKind)

	status, err = db.StepIn()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SingleStepTrap, status.TrapKind)

	status, err = db.StepIn()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, syscall.SIGTRAP, status.StopSignal)
	expect.Equal(t, SingleStepTrap, status.TrapKind)

	frames := db.CurrentThread().CallStack.ExecutingStack()
	expect.Equal(t, 4, len(frames))

	names := []string{}
	inlines := []bool{}
	for _, frame := range frames {
		names = append(names, frame.Name)
		inlines = append(inlines, frame.IsInlined())
	}

	expect.Equal(
		t,
		[]string{"scratch_ears", "pet_cat", "find_happiness", "main"},
		names)

	expect.Equal(
		t,
		[]bool{true, true, false, false},
		inlines)
}

func (DebuggerSuite) TestSharedLibraryTracing(t *testing.T) {
	cmd := exec.Command("test_targets/marshmallow")
	db, err := StartAndAttachTo(cmd)
	expect.Nil(t, err)
	defer db.Close()

	_, err = db.BreakPoints.Set(
		db.NewFunctionResolver("libmeow_client_is_cute"),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	status, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, SoftwareTrap, status.TrapKind)

	frames := db.CurrentThread().CallStack.ExecutingStack()
	expect.Equal(t, 2, len(frames))

	names := []string{}
	libs := []string{}
	for _, frame := range frames {
		names = append(names, frame.Name)

		fileName := frame.SourceFile.CompileUnit.File.FileName
		if fileName != "" {
			fileName = path.Base(fileName)
		}
		libs = append(libs, fileName)
	}

	expect.Equal(t, []string{"libmeow_client_is_cute", "main"}, names)
	expect.Equal(t, []string{"libmeow.so", ""}, libs)
}

func (DebuggerSuite) TestMultiThreading(t *testing.T) {
	cmd := exec.Command("test_targets/multi_threaded")
	db, err := StartAndAttachTo(cmd)
	expect.Nil(t, err)
	defer db.Close()

	created := map[int]struct{}{}
	exited := map[int]struct{}{}

	db.WatchThreadLifeCycle(
		func(status *ThreadStatus) {
			if status.Stopped {
				created[status.Tid] = struct{}{}
			} else if status.Exited {
				exited[status.Tid] = struct{}{}
			}
		})

	_, err = db.BreakPoints.Set(
		db.NewFunctionResolver("say_hi"),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	trapped := map[int]struct{}{}

	for db.MainThread().Status().Stopped {
		_, err := db.ResumeAllUntilSignal()
		expect.Nil(t, err)

		// NOTE: a single ResumeAllUntilSignal call may capture multiple trap
		// signals. Instead of checking only the returned / focus status, we need
		// to iterate through the entire threads list to look for all the traps
		_, list := db.ListThreads()
		for _, thread := range list {
			if thread.Status().TrapKind == SoftwareTrap {
				trapped[thread.Tid] = struct{}{}
			}
		}
	}

	expect.Equal(t, 10, len(created))
	expect.Equal(t, 10, len(trapped))
	expect.Equal(t, 10, len(exited))
	expect.True(t, db.MainThread().Status().Exited)
}

func (DebuggerSuite) TestDwarfExpression(t *testing.T) {
	instructions := []byte{
		// chunk 1
		byte(dwarf.DW_OP_reg16), byte(dwarf.DW_OP_piece), 4,
		// chunk 2
		byte(dwarf.DW_OP_piece), 8,
		// chunk 3
		byte(dwarf.DW_OP_const4u), 0xff, 0xff, 0xff, 0xff,
		byte(dwarf.DW_OP_bit_piece), 5, 12,
	}

	db, err := StartCmdAndAttachTo("test_targets/step")
	expect.Nil(t, err)
	defer db.Close()

	_, err = db.BreakPoints.Set(
		db.NewFunctionResolver("main"),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	status, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, SoftwareTrap, status.TrapKind)

	location, err := dwarf.EvaluateExpression(
		db.CurrentThread().CallStack.CurrentFrame(),
		false, // in frame info
		instructions,
		false) // push cfa
	expect.Nil(t, err)

	expect.Equal(t, 3, len(location))

	chunk := location[0]
	expect.Equal(t, dwarf.RegisterLocation, chunk.Kind)
	expect.Equal(t, 16, chunk.Value)
	expect.Equal(t, 4*8, chunk.BitSize)
	expect.Equal(t, 0, chunk.BitOffset)

	chunk = location[1]
	expect.Equal(t, dwarf.UnavailableLocation, chunk.Kind)
	expect.Equal(t, 8*8, chunk.BitSize)
	expect.Equal(t, 0, chunk.BitOffset)

	chunk = location[2]
	expect.Equal(t, dwarf.AddressLocation, chunk.Kind)
	expect.Equal(t, 0xffffffff, chunk.Value)
	expect.Equal(t, 5, chunk.BitSize)
	expect.Equal(t, 12, chunk.BitOffset)
}

func (DebuggerSuite) TestReadGlobalVariable(t *testing.T) {
	db, err := StartCmdAndAttachTo("test_targets/global_variable")
	expect.Nil(t, err)
	defer db.Close()

	_, err = db.BreakPoints.Set(
		db.NewFunctionResolver("main"),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	status, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, SoftwareTrap, status.TrapKind)

	checkVar := func(expected uint64) {
		globalVar, err := db.ResolveVariableExpression("g_int")
		expect.Nil(t, err)
		expect.Equal(t, UintKind, globalVar.Kind)
		expect.Equal(t, 8, globalVar.ByteSize)

		val, err := globalVar.DecodeSimpleValue()
		expect.Nil(t, err)
		expect.Equal(t, expected, val.(uint64))
	}

	checkVar(0)

	status, err = db.StepOver()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, SingleStepTrap, status.TrapKind)

	checkVar(1)

	status, err = db.StepOver()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, SingleStepTrap, status.TrapKind)

	checkVar(42)

	data, err := db.ResolveVariableExpression("ptr->pets[0].name")
	expect.Nil(t, err)
	expect.True(t, data.IsCharPointer())

	name, err := data.ReadCString()
	expect.Nil(t, err)
	expect.Equal(t, "Marshmallow", name)

	data, err = db.ResolveVariableExpression("sy.pets[2].name[3]")
	expect.Nil(t, err)
	expect.Equal(t, CharKind, data.Kind)
	expect.Equal(t, []byte("k"), data.Data)

	data, err = db.ResolveVariableExpression("cats[1].age")
	expect.Nil(t, err)
	expect.Equal(t, IntKind, data.Kind)
	expect.Equal(t, 4, data.ByteSize)

	age, err := data.DecodeSimpleValue()
	expect.Nil(t, err)
	expect.Equal(t, 8, age.(int32))

	data, err = db.ResolveVariableExpression("cats[1].color")
	expect.Nil(t, err)
	expect.Equal(t, IntKind, data.Kind)
	expect.Equal(t, 4, data.ByteSize)

	color, err := data.DecodeSimpleValue()
	expect.Nil(t, err)
	expect.Equal(t, 2, color.(int32))
}

func (DebuggerSuite) TestReadLocalVariable(t *testing.T) {
	db, err := StartCmdAndAttachTo("test_targets/blocks")
	expect.Nil(t, err)
	defer db.Close()

	_, err = db.BreakPoints.Set(
		db.NewFunctionResolver("main"),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	step := func() {
		status, err := db.StepOver()
		expect.Nil(t, err)
		expect.True(t, status.Stopped)
		expect.Equal(t, SingleStepTrap, status.TrapKind)
	}

	expects := func(expected int32) {
		data, err := db.ResolveVariableExpression("i")
		expect.Nil(t, err)
		expect.Equal(t, IntKind, data.Kind)
		expect.Equal(t, 4, data.ByteSize)

		i, err := data.DecodeSimpleValue()
		expect.Nil(t, err)
		expect.Equal(t, expected, i.(int32))
	}

	status, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, SoftwareTrap, status.TrapKind)

	step()
	step()

	expects(1)

	step()
	step()

	expects(2)

	step()
	step()

	expects(3)
}

func (DebuggerSuite) TestReadMemberPointer(t *testing.T) {
	db, err := StartCmdAndAttachTo("test_targets/member_pointer")
	expect.Nil(t, err)
	defer db.Close()

	_, err = db.BreakPoints.Set(
		db.NewLineResolver("member_pointer.cpp", 12),
		stoppoint.NewBreakSiteType(false),
		true)
	expect.Nil(t, err)

	status, err := db.ResumeAllUntilSignal()
	expect.Nil(t, err)
	expect.True(t, status.Stopped)
	expect.Equal(t, SoftwareTrap, status.TrapKind)

	data, err := db.ResolveVariableExpression("data_ptr")
	expect.Nil(t, err)
	expect.Equal(t, MemberPointerKind, data.Kind)
	expect.Equal(t, 8, data.ByteSize)

	dataAddr, err := data.DecodeSimpleValue()
	expect.Nil(t, err)
	expect.NotEqual(t, 0, dataAddr)

	data, err = db.ResolveVariableExpression("func_ptr")
	expect.Nil(t, err)
	expect.Equal(t, MemberPointerKind, data.Kind)
	expect.Equal(t, 16, data.ByteSize)

	funcAddr, err := data.DecodeSimpleValue()
	expect.Nil(t, err)
	expect.NotEqual(t, 0, funcAddr)
	expect.NotEqual(t, dataAddr, funcAddr)

}
