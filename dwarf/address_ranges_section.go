package dwarf

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pattyshack/bad/elf"
)

const (
	ElfDebugRangesSection = ".debug_ranges"

	baseAddressFlag = ^uint64(0)
)

type AddressRange struct {
	Low  elf.FileAddress
	High elf.FileAddress
}

func (addrRange AddressRange) Contains(addr elf.FileAddress) bool {
	return addrRange.Low <= addr && addr < addrRange.High
}

type AddressRanges []AddressRange

func (ranges AddressRanges) Contains(addr elf.FileAddress) bool {
	for _, addrRange := range ranges {
		if addrRange.Contains(addr) {
			return true
		}
	}
	return false
}

type AddressRangesSection struct {
	byteOrder binary.ByteOrder
	found     bool
	content   []byte
}

func NewAddressRangesSectionFromBytes(
	byteOrder binary.ByteOrder,
	content []byte,
) *AddressRangesSection {
	return &AddressRangesSection{
		byteOrder: byteOrder,
		found:     true,
		content:   content,
	}
}

func NewAddressRangesSection(file *elf.File) (*AddressRangesSection, error) {
	section := file.GetSection(ElfDebugRangesSection)

	var content []byte
	if section != nil {
		var err error
		content, err = section.RawContent()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to read elf .debug_ranges section: %w",
				err)
		}
	}

	return &AddressRangesSection{
		byteOrder: file.ByteOrder(),
		found:     section != nil,
		content:   content,
	}, nil
}

func (section *AddressRangesSection) AddressRangesAt(
	index SectionOffset,
	baseAddress elf.FileAddress,
) (
	AddressRanges,
	error,
) {
	if !section.found {
		return nil, fmt.Errorf("elf .debug_ranges section not found")
	}

	decode := NewCursor(section.byteOrder, section.content)
	_, err := decode.Seek(int(index), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("invalid address ranges index (%d): %w", index, err)
	}

	result := AddressRanges{}
	for !decode.HasReachedEnd() {
		low, err := decode.U64()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse address ranges. cannot decode low: %w",
				err)
		}

		high, err := decode.U64()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse address ranges. cannot decode high: %w",
				err)
		}

		if low == baseAddressFlag {
			baseAddress = elf.FileAddress(high)
			continue
		}

		if low == 0 && high == 0 {
			return result, nil
		}

		result = append(
			result,
			AddressRange{
				Low:  baseAddress + elf.FileAddress(low),
				High: baseAddress + elf.FileAddress(high),
			})
	}

	return nil, fmt.Errorf("address ranges (%d) not terminated", index)
}
