package common

import (
	"fmt"
)

var (
	ErrInvalidArgument = fmt.Errorf("invalid argument")
	ErrProcessExited   = fmt.Errorf("process exited")
)

type TrapKind string

const (
	UnknownTrap    = TrapKind("")
	SoftwareTrap   = TrapKind("software break")
	HardwareTrap   = TrapKind("hardware break")
	SingleStepTrap = TrapKind("single step")
	SyscallTrap    = TrapKind("syscall trap")
)

func TrapCodeToKind(code int32) TrapKind {
	// NOTE: on x64, linux incorrect report software trap as SI_KERNEL (0x80)
	// when it should have reported of TRAP_BRKPT (1).
	switch code {
	case 0x80: // SI_KERNEL
		return SoftwareTrap
	case 4: // TRAP_HWBKPT
		return HardwareTrap
	case 2: // TRAP_TRACE
		return SingleStepTrap
	default:
		// Most si_code values are not handled.  e.g, SI_TKILL (-6)
		return UnknownTrap
	}
}

type VirtualAddress uint64

func (addr VirtualAddress) String() string {
	return fmt.Sprintf("0x%016x", uint64(addr))
}

type VirtualAddresses []VirtualAddress

func (s VirtualAddresses) Len() int {
	return len(s)
}

func (s VirtualAddresses) Less(i int, j int) bool {
	return uint64(s[i]) < uint64(s[j])
}

func (s VirtualAddresses) Swap(i int, j int) {
	s[i], s[j] = s[j], s[i]
}

type AddressRange struct {
	Low  VirtualAddress
	High VirtualAddress
}

func (ar AddressRange) Contains(addr VirtualAddress) bool {
	return ar.Low <= addr && addr < ar.High
}

type AddressRanges []AddressRange

func (ars AddressRanges) Contains(addr VirtualAddress) bool {
	for _, ar := range ars {
		if ar.Contains(addr) {
			return true
		}
	}
	return false
}
