package dwarf

import (
	"github.com/pattyshack/bad/elf"
)

type SectionOffset int

type File struct {
	*elf.File

	*AbbreviationSection
	*InformationSection
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
		StringSection:        stringSection,
		AddressRangesSection: addressRangesSection,
	}
	infoSection.SetParent(file)

	return file, nil
}
