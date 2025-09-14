package ptrace

import (
	"golang.org/x/sys/unix"
	"syscall"
	"unsafe"
)

type Options int

const (
	vmPageSize = 0x1000

	O_EXITKILL     = Options(unix.PTRACE_O_EXITKILL)
	O_TRACESYSGOOD = Options(unix.PTRACE_O_TRACESYSGOOD)
)

// This matches user_regs_struct (64bit variant) defined in <sys/user.h>
type UserRegs = syscall.PtraceRegs

// This matches user_fpregs_struct (64bit variant) defined in <sys/user.h>
type UserFPRegs struct {
	Cwd      uint16 // Control
	Swd      uint16 // Status
	Ftw      uint16 // Tag
	Fop      uint16 // Last instruction opcode
	Rip      uint64 // Instruction pointer
	Rdp      uint64 // Data pointer
	Mxcsr    uint32 // MXCSR register state
	MxcrMask uint32 // MXCR mask

	// NOTE: c's st_space and xmm_space are defined as uint32 arrays.  We use
	// uint64 arrays here to simplify Uint128 representation.
	StSpace  [16]uint64 // 8*16 bytes for each FP-reg = 128 bytes
	XmmSpace [32]uint64 // 16*16 bytes for each XMM-reg = 256 bytes

	Padding [24]uint32
}

// This matches user (64bit variant) defined in <sys/user.h>
type User struct {
	Regs       UserRegs
	UFPValid   int
	I387       UserFPRegs
	UTSize     uint64
	UDSize     uint64
	USSize     uint64
	StartCode  uint64
	StartStack uint64
	Signal     int64
	Reserved   int
	UAr0       uintptr // struct user_regs_struct*
	UFPState   uintptr // struct user_fpregs_struct*
	Magic      uint64
	UComm      [32]byte
	UDebugReg  [8]uint64
}

type SigInfo = unix.Siginfo

func ptrace(request int, pid int, addr uintptr, data uintptr) error {
	_, _, err := syscall.Syscall6(
		syscall.SYS_PTRACE,
		uintptr(request),
		uintptr(pid),
		addr,
		data,
		0,
		0)
	if err == 0 {
		return nil
	}
	return err
}

func ptracePtr(request int, pid int, addr uintptr, data unsafe.Pointer) error {
	return ptrace(request, pid, addr, uintptr(data))
}

func getFPRegs(pid int, out *UserFPRegs) error {
	return ptracePtr(syscall.PTRACE_GETFPREGS, pid, 0, unsafe.Pointer(out))
}

func setFPRegs(pid int, in *UserFPRegs) error {
	return ptracePtr(syscall.PTRACE_SETFPREGS, pid, 0, unsafe.Pointer(in))
}

func peekUserArea(pid int, offset uintptr) (uintptr, error) {
	// Since we're issuing Syscall6 directly, we need to pass in a valid output
	// pointer.  See "C library/kernel differences" in ptrace man(2) page for
	// detail.
	data := uintptr(0)
	err := ptracePtr(syscall.PTRACE_PEEKUSR, pid, offset, unsafe.Pointer(&data))
	return data, err
}

func pokeUserArea(pid int, offset uintptr, data uintptr) error {
	return ptrace(syscall.PTRACE_POKEUSR, pid, offset, data)
}

func getSigInfo(pid int, out *SigInfo) error {
	return ptracePtr(syscall.PTRACE_GETSIGINFO, pid, 0, unsafe.Pointer(out))
}

func readVirtualMemory(pid int, addr uintptr, data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	localIovs := make([]unix.Iovec, 1)
	localIovs[0].Base = &data[0]
	localIovs[0].SetLen(len(data))

	var remoteIovs []unix.RemoteIovec

	remaining := len(data)

	// NOTE: We need to ensure RemoteIovec entries are page aligned.
	if addr%vmPageSize != 0 {
		pageEndAddr := ((addr + vmPageSize - 1) / vmPageSize) * vmPageSize

		size := int(pageEndAddr - addr)
		if remaining < size {
			size = remaining
		}

		remoteIovs = append(
			remoteIovs,
			unix.RemoteIovec{
				Base: addr,
				Len:  size,
			})
		remaining -= size
		addr += uintptr(size)
	}

	for remaining > 0 {
		size := remaining
		if size > vmPageSize {
			size = vmPageSize
		}

		remoteIovs = append(
			remoteIovs,
			unix.RemoteIovec{
				Base: addr,
				Len:  size,
			})

		remaining -= size
		addr += uintptr(size)
	}

	return unix.ProcessVMReadv(pid, localIovs, remoteIovs, 0)
}
