// NOTE: This implements gcc eh frame format instead of dwarf frame format.

package dwarf

import (
	"fmt"
	"sort"

	"github.com/pattyshack/bad/elf"
)

const (
	ehFrameV2 = 1 // dwarf2's version value is confusingly 1
	ehFrameV3 = 3
	ehFrameV4 = 4

	DW_EH_PE_absptr  = 0x00
	DW_EH_PE_uleb128 = 0x01
	DW_EH_PE_udata2  = 0x02
	DW_EH_PE_udata4  = 0x03
	DW_EH_PE_udata8  = 0x04
	DW_EH_PE_sleb128 = 0x09
	DW_EH_PE_sdata2  = 0x0a
	DW_EH_PE_sdata4  = 0x0b
	DW_EH_PE_sdata8  = 0x0c

	DW_EH_PE_pcrel   = 0x10
	DW_EH_PE_textrel = 0x20
	DW_EH_PE_datarel = 0x30
	DW_EH_PE_funcrel = 0x40
	DW_EH_PE_aligned = 0x50

	DW_EH_PE_indirect = 0x80
)

type FrameSection struct {
	*File

	ehFrameSectionStart    int64
	ehFrameHdrSectionStart int64
	textSectionStart       int64
	gotPltSectionStart     int64 // 0 if the section is missing

	fdes []*FrameDescriptionEntry
}

func (section *FrameSection) SetParent(file *File) {
	section.File = file
}

func (section *FrameSection) FDEContainingAddress(
	address elf.FileAddress,
) *FrameDescriptionEntry {

	fdes := section.fdes
	if len(fdes) == 0 || address < fdes[0].Low {
		return nil
	}

	for len(fdes) > 2 {
		midIdx := len(fdes) / 2

		mid := fdes[midIdx]
		if address < mid.Low {
			fdes = fdes[:midIdx]
		} else if address == mid.Low {
			return mid
		} else {
			fdes = fdes[midIdx+1:]
		}
	}

	for _, entry := range fdes {
		if entry.Contains(address) {
			return entry
		}
	}

	return nil
}

func (section *FrameSection) ComputeUnwindRulesAt(
	address elf.FileAddress,
) (
	*UnwindRules,
	error,
) {
	fde := section.FDEContainingAddress(address)
	if fde == nil {
		return nil, nil
	}

	return computeUnwindRules(fde, address)
}

func NewFrameSection(file *elf.File) (*FrameSection, error) {
	section := file.GetSection(ElfEhFrameSection)
	if section == nil {
		return nil, fmt.Errorf("elf .eh_frame %w", ErrSectionNotFound)
	}
	ehFrameSectionStart := int64(section.Header().Offset)

	content, err := section.RawContent()
	if err != nil {
		return nil, fmt.Errorf("failed to read elf .eh_frame section: %w", err)
	}

	section = file.GetSection(ElfEhFrameHdrSection)
	if section == nil {
		return nil, fmt.Errorf("elf .eh_frame_hdr %w", ErrSectionNotFound)
	}
	ehFrameHdrSectionStart := int64(section.Header().Offset)

	section = file.GetSection(ElfTextSection)
	if section == nil {
		return nil, fmt.Errorf("elf .text %w", ErrSectionNotFound)
	}
	textSectionStart := int64(section.Header().Offset)

	gotPltSectionStart := int64(0)
	section = file.GetSection(ElfGotPltSection)
	if section != nil {
		gotPltSectionStart = int64(section.Header().Offset)
	}

	frameSection := &FrameSection{
		ehFrameSectionStart:    ehFrameSectionStart,
		ehFrameHdrSectionStart: ehFrameHdrSectionStart,
		textSectionStart:       textSectionStart,
		gotPltSectionStart:     gotPltSectionStart,
	}

	decoder := newFrameEntryDecoder(
		frameSection,
		NewCursor(file.ByteOrder(), content))
	parse := frameParser{
		framePointerDecoder: decoder,
		cies:                map[SectionOffset]*CommonInfoEntry{},
		FrameSection:        frameSection,
	}

	for !parse.HasReachedEnd() {
		err := parse.frameEntry()
		if err != nil {
			return nil, err
		}
	}

	sort.Slice(
		parse.fdes,
		func(i int, j int) bool {
			return parse.fdes[i].AddressRange.Low < parse.fdes[j].AddressRange.Low
		})

	return frameSection, nil
}

type CommonInfoEntry struct {
	*FrameSection

	SectionOffset

	CodeAlignmentFactor uint64
	DataAlignmentFactor int64

	HasAugmentation bool
	PointerEncoding uint8

	InstructionsStart SectionOffset
	Instructions      []byte
}

type FrameDescriptionEntry struct {
	SectionOffset

	*CommonInfoEntry

	AddressRange

	InstructionsStart SectionOffset
	Instructions      []byte
}

type frameParser struct {
	*framePointerDecoder

	cies map[SectionOffset]*CommonInfoEntry

	*FrameSection
}

func (parse *frameParser) frameEntry() error {
	start := parse.Position

	size, err := parse.U32()
	if err != nil {
		return fmt.Errorf("failed to parse frame entry. invalid size: %w", err)
	}
	if size == ^uint32(0) {
		return fmt.Errorf(
			"failed to parse frame entry. 64-bit dwarf format not supported.")
	}
	if size == 0 && parse.HasReachedEnd() { // last entry in the eh frame section
		return nil
	}

	end := parse.Position + int(size)

	cieStart := parse.Position
	cieDelta, err := parse.S32()
	if err != nil {
		return fmt.Errorf("failed to parse frame entry. invalid cie delta: %w", err)
	}

	// NOTE: eh format uses 0 to indicate common info entry, whereas dwarf format
	// uses 0xffffffff
	if cieDelta == 0 {
		cie, err := parse.commonInfoEntry(start, end)
		if err != nil {
			return fmt.Errorf("failed to parse common info entry: %w", err)
		}

		parse.cies[cie.SectionOffset] = cie
	} else {
		fde, err := parse.frameDescriptionEntry(
			start,
			end,
			SectionOffset(cieStart-int(cieDelta)))
		if err != nil {
			return fmt.Errorf("failed to parse frame description entry: %w", err)
		}

		parse.fdes = append(parse.fdes, fde)
	}

	return nil
}

func (parse *frameParser) commonInfoEntry(
	start int,
	end int,
) (
	*CommonInfoEntry,
	error,
) {
	version, err := parse.U8()
	if err != nil {
		return nil, fmt.Errorf("invalid version: %w", err)
	}
	switch version {
	case ehFrameV2, ehFrameV3, ehFrameV4:
	default:
		return nil, fmt.Errorf("eh frame version %d not supported", version)
	}

	// NOTE: eh format only
	augmentationString, err := parse.String()
	if err != nil {
		return nil, fmt.Errorf("invalid augmentation string: %w", err)
	}

	if version == ehFrameV4 {
		addressSize, err := parse.U8()
		if err != nil {
			return nil, fmt.Errorf("invalid address size: %w", err)
		}
		if addressSize != 8 {
			return nil, fmt.Errorf("address size %d not supported", addressSize)
		}

		segmentSize, err := parse.U8()
		if err != nil {
			return nil, fmt.Errorf("invalid segment size: %w", err)
		}
		if segmentSize != 0 {
			return nil, fmt.Errorf("segment size %d not supported", addressSize)
		}
	}

	codeAlignmentFactor, err := parse.ULEB128(64)
	if err != nil {
		return nil, fmt.Errorf("invalid code alignment factor: %w", err)
	}

	dataAlignmentFactor, err := parse.SLEB128(64)
	if err != nil {
		return nil, fmt.Errorf("invalid data alignment factor: %w", err)
	}

	var returnAddressRegister uint64
	if version == ehFrameV2 {
		reg, err := parse.U8()
		if err != nil {
			return nil, fmt.Errorf("invalid return address register: %w", err)
		}
		returnAddressRegister = uint64(reg)
	} else {
		returnAddressRegister, err = parse.ULEB128(64)
		if err != nil {
			return nil, fmt.Errorf("invalid return address register: %w", err)
		}
	}
	if returnAddressRegister != 16 { // i.e., rip / program counter
		if err != nil {
			return nil, fmt.Errorf(
				"unsupported return address register (%d) on x64",
				returnAddressRegister)
		}
	}

	// augmentation data array (eh format only)
	augmentationDataStart := 0
	augmentationDataSize := 0
	var pointerEncoding uint8
	for idx, char := range []byte(augmentationString) {
		if idx == 0 && char != 'z' {
			return nil, fmt.Errorf("invalid augmentation (%s)", augmentationString)
		}

		switch char {
		case 'z':
			if idx != 0 {
				return nil, fmt.Errorf("malformed augmentation")
			}

			size, err := parse.ULEB128(31)
			if err != nil {
				return nil, fmt.Errorf("invalid augmentation size: %w", err)
			}
			augmentationDataStart = parse.Position
			augmentationDataSize = int(size)
		case 'R':
			encoding, err := parse.U8()
			if err != nil {
				return nil, fmt.Errorf("invalid fde pointer encoding: %w", err)
			}
			pointerEncoding = encoding
		case 'L':
			// language specific data area pointer encoding (not used by the debugger)
			_, err := parse.U8()
			if err != nil {
				return nil, fmt.Errorf("invalid language pointer encoding: %w", err)
			}
		case 'P':
			// personality pointer (not used by the debugger)
			encoding, err := parse.U8()
			if err != nil {
				return nil, fmt.Errorf("invalid personality pointer encoding: %w", err)
			}

			_, err = parse.framePointer(encoding)
			if err != nil {
				return nil, fmt.Errorf("invalid personality pointer: %w", err)
			}
		}
	}

	hasAugmentation := len(augmentationString) != 0
	if hasAugmentation {
		size := parse.Position - augmentationDataStart
		if augmentationDataSize != size {
			return nil, fmt.Errorf(
				"incorrect augmentation data size (%d != %d)",
				augmentationDataSize,
				size)
		}
	}

	instructionsStart := SectionOffset(parse.Position)
	instructions, err := parse.Bytes(end - parse.Position)
	if err != nil {
		return nil, fmt.Errorf("invalid instructions: %w", err)
	}

	return &CommonInfoEntry{
		FrameSection:        parse.FrameSection,
		SectionOffset:       SectionOffset(start),
		CodeAlignmentFactor: codeAlignmentFactor,
		DataAlignmentFactor: dataAlignmentFactor,
		HasAugmentation:     hasAugmentation,
		PointerEncoding:     pointerEncoding,
		InstructionsStart:   instructionsStart,
		Instructions:        instructions,
	}, nil
}

func (parse *frameParser) frameDescriptionEntry(
	start int,
	end int,
	cieId SectionOffset,
) (
	*FrameDescriptionEntry,
	error,
) {
	cie, ok := parse.cies[cieId]
	if !ok {
		return nil, fmt.Errorf("common info entry (%d) not found", cieId)
	}

	lowAddress, err := parse.framePointer(cie.PointerEncoding)
	if err != nil {
		return nil, fmt.Errorf("invalid initial location address: %w", err)
	}

	// NOTE: delta (aka FDE address_range) uses the same offset encoding as
	// regular cie.PointerEncoding, but always uses 0 (absptr) as base.
	offsetEncoding := cie.PointerEncoding & 0x0f
	deltaEncoding := DW_EH_PE_absptr | offsetEncoding

	delta, err := parse.framePointer(deltaEncoding)
	if err != nil {
		return nil, fmt.Errorf("invalid address range: %w", err)
	}

	instructionsStart := SectionOffset(parse.Position)
	instructions, err := parse.Bytes(end - parse.Position)
	if err != nil {
		return nil, fmt.Errorf("invalid instructions: %w", err)
	}

	return &FrameDescriptionEntry{
		SectionOffset:   SectionOffset(start),
		CommonInfoEntry: cie,
		AddressRange: AddressRange{
			Low:  lowAddress,
			High: lowAddress + delta,
		},
		InstructionsStart: instructionsStart,
		Instructions:      instructions,
	}, nil
}

type framePointerDecoder struct {
	cursorStart int64 // relative to the beginning of the elf file
	*Cursor

	// The start of the .text section
	textStart int64

	// When decoding .eh_frame_hdr entries, dataStart is the start of the
	// .eh_frame_hdr section.
	//
	// When decoding .eh_frame entries, dataStart is 0.
	//
	// When decoding CIE/FDE instructions, dataStart is either the start of
	// the .got.plt section, or 0 if the section is missing.
	dataStart int64

	// When parsing cie/fde instructions, FuncStart is fde.AddressRange.Low.
	// Otherwise, FuncStart is zero.
	funcStart int64
}

func newFrameEntryDecoder(
	section *FrameSection,
	cursor *Cursor,
) *framePointerDecoder {
	return &framePointerDecoder{
		cursorStart: section.ehFrameSectionStart,
		Cursor:      cursor,
		textStart:   section.textSectionStart,
		dataStart:   0,
		funcStart:   0,
	}
}

func newCIEInstructionDecoder(
	state *cfiState,
) *framePointerDecoder {
	fde := state.FrameDescriptionEntry

	cursorStart := fde.ehFrameSectionStart
	cursorStart += int64(fde.CommonInfoEntry.InstructionsStart)

	cursor := NewCursor(fde.ByteOrder(), fde.CommonInfoEntry.Instructions)

	return &framePointerDecoder{
		cursorStart: cursorStart,
		Cursor:      cursor,
		textStart:   fde.textSectionStart,
		dataStart:   fde.gotPltSectionStart,
		funcStart:   int64(fde.AddressRange.Low),
	}
}

func newFDEInstructionDecoder(
	state *cfiState,
) *framePointerDecoder {
	fde := state.FrameDescriptionEntry

	cursorStart := fde.ehFrameSectionStart
	cursorStart += int64(fde.InstructionsStart)

	cursor := NewCursor(fde.ByteOrder(), fde.Instructions)

	return &framePointerDecoder{
		cursorStart: cursorStart,
		Cursor:      cursor,
		textStart:   fde.textSectionStart,
		dataStart:   fde.gotPltSectionStart,
		funcStart:   int64(fde.AddressRange.Low),
	}
}

func (decode *framePointerDecoder) framePointer(
	encoding uint8,
) (
	elf.FileAddress,
	error,
) {
	ptr, err := decode._framePointer(encoding)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to parse eh frame pointer: %w",
			err)
	}

	return ptr, nil
}

func (decode *framePointerDecoder) _framePointer(
	encoding uint8,
) (
	elf.FileAddress,
	error,
) {
	base := int64(0)
	switch encoding & 0x70 {
	case DW_EH_PE_absptr:
		// do nothing
	case DW_EH_PE_pcrel:
		base = decode.cursorStart + int64(decode.Position)
	case DW_EH_PE_textrel:
		base = decode.textStart
	case DW_EH_PE_datarel:
		base = decode.dataStart
	case DW_EH_PE_funcrel:
		base = decode.funcStart
	default:
		return 0, fmt.Errorf("unsupported eh frame pointer encoding (%d)", encoding)
	}

	offset := int64(0)

	switch encoding & 0xf {
	case DW_EH_PE_absptr, DW_EH_PE_udata8:
		off, err := decode.U64()
		if err != nil {
			return 0, err
		}
		offset = int64(off)
	case DW_EH_PE_uleb128:
		off, err := decode.ULEB128(31)
		if err != nil {
			return 0, err
		}
		offset = int64(off)
	case DW_EH_PE_udata2:
		off, err := decode.U16()
		if err != nil {
			return 0, err
		}
		offset = int64(off)
	case DW_EH_PE_udata4:
		off, err := decode.U32()
		if err != nil {
			return 0, err
		}
		offset = int64(off)
	case DW_EH_PE_sleb128:
		off, err := decode.SLEB128(32)
		if err != nil {
			return 0, err
		}
		offset = off
	case DW_EH_PE_sdata2:
		off, err := decode.S16()
		if err != nil {
			return 0, err
		}
		offset = int64(off)
	case DW_EH_PE_sdata4:
		off, err := decode.S32()
		if err != nil {
			return 0, err
		}
		offset = int64(off)
	case DW_EH_PE_sdata8:
		off, err := decode.S64()
		if err != nil {
			return 0, err
		}
		offset = off
	default:
		return 0, fmt.Errorf("unsupported eh frame pointer encoding (%d)", encoding)
	}

	addr := base + offset
	if addr < 0 {
		return 0, fmt.Errorf("negative file addr (%d)", addr)
	}

	return elf.FileAddress(addr), nil
}
