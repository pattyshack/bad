package dwarf

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pattyshack/bad/elf"
)

type LocationSection struct {
	byteOrder binary.ByteOrder
	found     bool
	content   []byte
}

func NewLocationSection(file *elf.File) (*LocationSection, error) {
	section := file.GetSection(ElfDebugLocationSection)

	var content []byte
	if section != nil {
		var err error
		content, err = section.RawContent()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to read elf .debug_loc section: %w",
				err)
		}
	}

	return &LocationSection{
		byteOrder: file.ByteOrder(),
		found:     section != nil,
		content:   content,
	}, nil
}

func (section *LocationSection) EvaluateLocation(
	index SectionOffset,
	baseAddress elf.FileAddress, // compile unit root's low address
	context ExpressionContext,
	inFrameInfo bool,
) (
	Location,
	error,
) {
	if !section.found {
		return nil, nil
	}

	baseFileAddr := uint64(baseAddress)
	pcFileAddr := context.ProgramCounter() - context.LoadBias()

	decode := NewCursor(section.byteOrder, section.content)
	_, err := decode.Seek(int(index), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("invalid location index (%d): %w", index, err)
	}

	for !decode.HasReachedEnd() {
		low, err := decode.U64()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse location entry. cannot decode low: %w",
				err)
		}

		high, err := decode.U64()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse location entry. cannot decode high: %w",
				err)
		}

		if low == baseAddressFlag {
			baseFileAddr = high
			continue
		}

		if low == 0 && high == 0 { // entry not found at end of list
			return nil, nil
		}

		length, err := decode.U16()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse location entry. cannot decode instructions length: %w",
				err)
		}

		instructions, err := decode.Bytes(int(length))
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse location entry. cannot decode instructions bytes: %w",
				err)
		}

		if baseFileAddr+low <= pcFileAddr && pcFileAddr < baseFileAddr+high {
			return EvaluateExpression(context, inFrameInfo, instructions, false)
		}
	}

	return nil, fmt.Errorf("location list (%d) not terminated", index)
}
