package memory

import (
	"bytes"
	"fmt"

	"golang.org/x/arch/x86/x86asm"

	. "github.com/pattyshack/bad/debugger/common"
)

const (
	maxX64InstructionLength = 15
)

var (
	endbr64 = []byte{0xf3, 0x0f, 0x01e, 0xfa}
	endbr32 = []byte{0xf3, 0x0f, 0x01e, 0xfb}
)

type DisassembledInstruction struct {
	Address VirtualAddress

	IsEndbr64 bool
	IsEndbr32 bool

	x86asm.Inst
}

func (inst DisassembledInstruction) String() string {
	if inst.IsEndbr64 {
		return fmt.Sprintf("0x%016x: endbr64", uint64(inst.Address))
	} else if inst.IsEndbr32 {
		return fmt.Sprintf("0x%016x: endbr32", uint64(inst.Address))
	}

	return fmt.Sprintf(
		"0x%016x: %s",
		uint64(inst.Address),
		x86asm.GNUSyntax(inst.Inst, uint64(inst.Address), nil))
}

type StopSiteBytes interface {
	// If an enabled stop site is in the range
	//    [startAddr, startAddr + len(memorySlice))
	// replace the stop site bytes with the original data bytes in the
	// memorySlice.
	ReplaceStopSiteBytes(startAddr VirtualAddress, memorySlice []byte)
}

type Disassembler struct {
	memory    *VirtualMemory
	stopSites StopSiteBytes
}

func NewDisassembler(
	memory *VirtualMemory,
	stopSites StopSiteBytes,
) *Disassembler {
	return &Disassembler{
		memory:    memory,
		stopSites: stopSites,
	}
}

func (disassembler *Disassembler) Disassemble(
	startAddress VirtualAddress,
	numInstructions int,
) (
	[]DisassembledInstruction,
	error,
) {
	if numInstructions < 0 {
		return nil, fmt.Errorf(
			"Invalid number of instructions to disassemble: %d",
			numInstructions)
	} else if numInstructions == 0 {
		return nil, nil
	}

	data := make([]byte, numInstructions*maxX64InstructionLength)
	_, err := disassembler.memory.Read(startAddress, data)
	if err != nil {
		return nil, err
	}

	disassembler.stopSites.ReplaceStopSiteBytes(startAddress, data)

	address := startAddress
	result := make([]DisassembledInstruction, 0, numInstructions)
	for len(data) > 0 && len(result) < numInstructions {
		var inst x86asm.Inst
		isEndbr64 := false
		isEndbr32 := false
		length := 0
		if len(data) >= len(endbr64) &&
			bytes.Equal(data[:len(endbr64)], endbr64) {

			isEndbr64 = true
			length = len(endbr64)
		} else if len(data) >= len(endbr32) &&
			bytes.Equal(data[:len(endbr32)], endbr32) {

			isEndbr32 = true
			length = len(endbr32)
		} else {
			var err error
			inst, err = x86asm.Decode(data, 64)
			if err != nil {
				break
			}

			length = inst.Len
		}

		result = append(
			result,
			DisassembledInstruction{
				Address:   address,
				IsEndbr64: isEndbr64,
				IsEndbr32: isEndbr32,
				Inst:      inst,
			})

		data = data[length:]
		address += VirtualAddress(length)
	}

	return result, nil
}
