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

type EhCommonInfoEntry struct {
	SectionOffset

	CodeAlignmentFactor uint64
	DataAlignmentFactor uint64

	HasAugmentation bool
	PointerEncoding uint8

	Instructions []byte
}

type EhFrameDescriptionEntry struct {
	SectionOffset

	*EhCommonInfoEntry

	AddressRange

	Instructions []byte
}

type EhFramePointer struct {
	// When IsTextRelative is true, the Offset is relative to the start of the
	// .text section. Otherwise, Offset is either an absolute FileAddress, or an
	// offset relative to the start of the function's FileAddress.
	Offset uint64

	IsTextRelative bool

	// Only applicable when the pointer is a FileAddress.
	IsFunctionRelative bool
}

type EhFrameSection struct {
	entries []*EhFrameDescriptionEntry
}

func (section *EhFrameSection) EhFrameContainingAddress(
	address elf.FileAddress,
) *EhFrameDescriptionEntry {

	entries := section.entries
	if len(entries) == 0 || address < entries[0].Low {
		return nil
	}

	for len(entries) > 2 {
		midIdx := len(entries) / 2

		mid := entries[midIdx]
		if address < mid.Low {
			entries = entries[:midIdx]
		} else if address == mid.Low {
			return mid
		} else {
			entries = entries[midIdx+1:]
		}
	}

	for _, entry := range entries {
		if entry.Contains(address) {
			return entry
		}
	}

	return nil
}

func NewEhFrameSection(file *elf.File) (*EhFrameSection, error) {
	section := file.GetSection(ElfEhFrameSection)
	if section == nil {
		return nil, fmt.Errorf("elf .eh_frame %w", ErrSectionNotFound)
	}

	content, err := section.RawContent()
	if err != nil {
		return nil, fmt.Errorf("failed to read elf .eh_frame section: %w", err)
	}

	cursor := NewCursor(file.ByteOrder(), content)

	parse := ehFrameParser{
		currentSectionOffset: int64(section.Header().Offset),
		Cursor:               cursor,
		cies:                 map[SectionOffset]*EhCommonInfoEntry{},
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

	return &EhFrameSection{
		entries: parse.fdes,
	}, nil
}

type ehFrameParser struct {
	currentSectionOffset int64
	*Cursor

	cies map[SectionOffset]*EhCommonInfoEntry
	fdes []*EhFrameDescriptionEntry
}

func (parse *ehFrameParser) frameEntry() error {
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

func (parse *ehFrameParser) commonInfoEntry(
	start int,
	end int,
) (
	*EhCommonInfoEntry,
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

	dataAlignmentFactor, err := parse.ULEB128(64)
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

	instructions, err := parse.Bytes(end - parse.Position)
	if err != nil {
		return nil, fmt.Errorf("invalid instructions: %w", err)
	}

	return &EhCommonInfoEntry{
		SectionOffset:       SectionOffset(start),
		CodeAlignmentFactor: codeAlignmentFactor,
		DataAlignmentFactor: dataAlignmentFactor,
		HasAugmentation:     hasAugmentation,
		PointerEncoding:     pointerEncoding,
		Instructions:        instructions,
	}, nil
}

func (parse *ehFrameParser) frameDescriptionEntry(
	start int,
	end int,
	cieId SectionOffset,
) (
	*EhFrameDescriptionEntry,
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
	if lowAddress.IsTextRelative || lowAddress.IsFunctionRelative {
		// Expecting absolute file address.
		return nil, fmt.Errorf(
			"unexpected initial location address pointer (%v)",
			lowAddress)
	}

	delta, err := parse.framePointer(cie.PointerEncoding)
	if err != nil {
		return nil, fmt.Errorf("invalid address range: %w", err)
	}
	if delta.IsTextRelative || delta.IsFunctionRelative {
		// Expecting absolute file address.
		return nil, fmt.Errorf("unexpected address range pointer (%v)", delta)
	}

	instructions, err := parse.Bytes(end - parse.Position)
	if err != nil {
		return nil, fmt.Errorf("invalid instructions: %w", err)
	}

	return &EhFrameDescriptionEntry{
		SectionOffset:     SectionOffset(start),
		EhCommonInfoEntry: cie,
		AddressRange: AddressRange{
			Low:  elf.FileAddress(lowAddress.Offset),
			High: elf.FileAddress(lowAddress.Offset + delta.Offset),
		},
		Instructions: instructions,
	}, nil
}

func (parse *ehFrameParser) framePointer(
	encoding uint8,
) (
	EhFramePointer,
	error,
) {
	ptr, err := parse._parseEhFramePointer(encoding)
	if err != nil {
		return EhFramePointer{}, fmt.Errorf(
			"failed to parse eh frame pointer: %w",
			err)
	}

	return ptr, nil
}

func (parse *ehFrameParser) _parseEhFramePointer(
	encoding uint8,
) (
	EhFramePointer,
	error,
) {
	base := int64(0)
	isTextRelative := false
	isFunctionRelative := false
	switch encoding & 0x70 {
	case DW_EH_PE_absptr:
		// do nothing
	case DW_EH_PE_pcrel:
		base = parse.currentSectionOffset + int64(parse.Position)
	case DW_EH_PE_textrel:
		isTextRelative = true
	case DW_EH_PE_datarel:
		// NOTE: When parsing entries from .eh_frame_hdr, the pointer is relative
		// to the .eh_frame_hdr section start. For everything else, the pointer is
		// the absolute location.
		//
		// Do nothing since aren't parsing .eh_frame_hdr.
	case DW_EH_PE_funcrel:
		isFunctionRelative = true
	default:
		return EhFramePointer{}, fmt.Errorf(
			"unsupported eh frame pointer encoding (%d)",
			encoding)
	}

	delta := int64(0)

	switch encoding & 0xf {
	case DW_EH_PE_absptr, DW_EH_PE_udata8:
		off, err := parse.U64()
		if err != nil {
			return EhFramePointer{}, err
		}
		delta = int64(off)
	case DW_EH_PE_uleb128:
		off, err := parse.ULEB128(31)
		if err != nil {
			return EhFramePointer{}, err
		}
		delta = int64(off)
	case DW_EH_PE_udata2:
		off, err := parse.U16()
		if err != nil {
			return EhFramePointer{}, err
		}
		delta = int64(off)
	case DW_EH_PE_udata4:
		off, err := parse.U32()
		if err != nil {
			return EhFramePointer{}, err
		}
		delta = int64(off)
	case DW_EH_PE_sleb128:
		off, err := parse.SLEB128(32)
		if err != nil {
			return EhFramePointer{}, err
		}
		delta = off
	case DW_EH_PE_sdata2:
		off, err := parse.S16()
		if err != nil {
			return EhFramePointer{}, err
		}
		delta = int64(off)
	case DW_EH_PE_sdata4:
		off, err := parse.S32()
		if err != nil {
			return EhFramePointer{}, err
		}
		delta = int64(off)
	case DW_EH_PE_sdata8:
		off, err := parse.S64()
		if err != nil {
			return EhFramePointer{}, err
		}
		delta = off
	default:
		return EhFramePointer{}, fmt.Errorf(
			"unsupported eh frame pointer encoding (%d)",
			encoding)
	}

	offset := base + delta
	if offset < 0 {
		return EhFramePointer{}, fmt.Errorf("negative section offset (%d)", offset)
	}

	return EhFramePointer{
		Offset:             uint64(offset),
		IsTextRelative:     isTextRelative,
		IsFunctionRelative: isFunctionRelative,
	}, nil
}
