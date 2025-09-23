package memory

import (
	"fmt"

	"golang.org/x/arch/x86/x86asm"

	. "github.com/pattyshack/bad/debugger/common"
)

const (
	maxX64InstructionLength = 15
)

type DisassembledInstruction struct {
	Address VirtualAddress
	x86asm.Inst
}

func (inst DisassembledInstruction) String() string {
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
		inst, err := x86asm.Decode(data, 64)
		if err != nil {
			break
		}

		result = append(
			result,
			DisassembledInstruction{
				Address: address,
				Inst:    inst,
			})

		data = data[inst.Len:]
		address += VirtualAddress(inst.Len)
	}

	return result, nil
}
