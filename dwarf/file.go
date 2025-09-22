package dwarf

import (
	"github.com/pattyshack/bad/elf"
)

type SectionOffset int

type File struct {
	*elf.File

	// NOTE: string, address ranges and line sections are optional
	*StringSection
	*AddressRangesSection
	*LineSection

	// NOTE: abbreviation and information sections are required
	*AbbreviationSection
	*InformationSection
}

func NewFile(elfFile *elf.File) (*File, error) {
	stringSection, err := NewStringSection(elfFile)
	if err != nil {
		return nil, err
	}

	addressRangesSection, err := NewAddressRangesSection(elfFile)
	if err != nil {
		return nil, err
	}

	lineSection, err := NewLineSection(elfFile)
	if err != nil {
		return nil, err
	}

	abbrevSection, err := NewAbbreviationSection(elfFile)
	if err != nil {
		return nil, err
	}

	infoSection, err := NewInformationSection(elfFile)
	if err != nil {
		return nil, err
	}

	file := &File{
		File:                 elfFile,
		StringSection:        stringSection,
		AddressRangesSection: addressRangesSection,
		LineSection:          lineSection,
		AbbreviationSection:  abbrevSection,
		InformationSection:   infoSection,
	}
	infoSection.SetParent(file)

	return file, nil
}
