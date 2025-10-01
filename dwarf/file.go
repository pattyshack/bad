package dwarf

import (
	"fmt"

	"github.com/pattyshack/bad/elf"
)

var (
	ErrSectionNotFound = fmt.Errorf("section not found")

	ElfDebugAbbreviationSection = ".debug_abbrev"
	ElfDebugRangesSection       = ".debug_ranges"
	ElfDebugInformationSection  = ".debug_info"
	ElfDebugLineSection         = ".debug_line"
	ElfDebugStringSection       = ".debug_str"

	ElfEhFrameSection    = ".eh_frame"
	ElfEhFrameHdrSection = ".eh_frame_hdr"
	ElfTextSection       = ".text"
	ElfGotPltSection     = ".got.plt"
)

type SectionOffset int

type File struct {
	*elf.File

	// Required
	*AbbreviationSection
	*InformationSection
	*LineSection
	*FrameSection

	// Optional
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

	ehFrameSection, err := NewFrameSection(elfFile)
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
		FrameSection:         ehFrameSection,
		StringSection:        stringSection,
		AddressRangesSection: addressRangesSection,
	}
	infoSection.SetParent(file)
	ehFrameSection.SetParent(file)

	return file, nil
}
