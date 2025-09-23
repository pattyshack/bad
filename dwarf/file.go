package dwarf

import (
	"fmt"

	"github.com/pattyshack/bad/elf"
)

var (
	ErrSectionNotFound = fmt.Errorf("section not found")
)

type SectionOffset int

type File struct {
	*elf.File

	// NOTE: abbreviation, information, and line sections are required
	*AbbreviationSection
	*InformationSection
	*LineSection

	// NOTE: string, address ranges and line sections are optional
	*StringSection
	*AddressRangesSection
}

func NewFile(elfFile *elf.File) (*File, error) {
	abbrevSection, err := NewAbbreviationSection(elfFile)
	if err != nil {
		return nil, err
	}

	infoSection, err := NewInformationSection(elfFile)
	if err != nil {
		return nil, err
	}

	lineSection, err := NewLineSection(elfFile)
	if err != nil {
		return nil, err
	}

	stringSection, err := NewStringSection(elfFile)
	if err != nil {
		return nil, err
	}

	addressRangesSection, err := NewAddressRangesSection(elfFile)
	if err != nil {
		return nil, err
	}

	file := &File{
		File:                 elfFile,
		AbbreviationSection:  abbrevSection,
		InformationSection:   infoSection,
		LineSection:          lineSection,
		StringSection:        stringSection,
		AddressRangesSection: addressRangesSection,
	}
	infoSection.SetParent(file)

	return file, nil
}
