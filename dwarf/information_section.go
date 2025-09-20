package dwarf

import (
	"fmt"

	"github.com/pattyshack/bad/elf"
)

const (
	ElfDebugInformationSection = ".debug_info"
)

type ProcessFunc func(*DebugInfoEntry) error

type CompileUnit struct {
	*File
	Start        SectionOffset
	ContentStart SectionOffset
	End          SectionOffset

	AbbreviationIndex SectionOffset
	Content           []byte

	// nil indicates the compile unit's content has not been parsed yet.
	root    *DebugInfoEntry
	entries []*DebugInfoEntry
}

func parseCompileUnit(
	decode *Cursor,
) (
	*CompileUnit,
	error,
) {
	start := SectionOffset(decode.Position)

	size, err := decode.U32()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse compile unit. invalid size: %w",
			err)
	}
	if size == ^uint32(0) {
		return nil, fmt.Errorf(
			"failed to parse compile unit. 64-bit dwarf format not supported")
	}

	version, err := decode.U16()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse compile unit. invalid version: %w",
			err)
	}
	if version != 4 {
		return nil, fmt.Errorf(
			"failed to parse compile unit. dwarf version %d not supported",
			version)
	}

	abbrevIndex, err := decode.U32()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse compile unit. invalid abbreviation index: %w",
			err)
	}

	addrSize, err := decode.U8()
	if addrSize != 8 {
		return nil, fmt.Errorf(
			"failed to parse compile unit. address size %d not supported",
			addrSize)
	}

	// NOTE: size does not include the size field itself (4-bytes), but
	// include other header fields
	// size = len(version + abbrevOffset + addrSize) + len(content)
	//      = 7 + len(content)
	contentLength := int(size) - 7
	if contentLength < 0 {
		return nil, fmt.Errorf(
			"failed to parse compile unit. invalid content length (%d)",
			contentLength)
	}

	contentStart := SectionOffset(decode.Position)

	unitContent, err := decode.Bytes(contentLength)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse compile unit. invalid content: %w",
			err)
	}

	return &CompileUnit{
		Start:             start,
		ContentStart:      contentStart,
		End:               SectionOffset(decode.Position),
		AbbreviationIndex: SectionOffset(abbrevIndex),
		Content:           unitContent,
	}, nil
}

func (unit *CompileUnit) Contains(offset SectionOffset) bool {
	return unit.Start <= offset && offset < unit.End
}

func (unit *CompileUnit) Root() (*DebugInfoEntry, error) {
	err := unit.maybeParseDebugInfoEntries()
	if err != nil {
		return nil, err
	}

	return unit.root, nil
}

func (unit *CompileUnit) DebugInfoEntries() ([]*DebugInfoEntry, error) {
	err := unit.maybeParseDebugInfoEntries()
	if err != nil {
		return nil, err
	}

	return unit.entries, nil
}

func (unit *CompileUnit) EntryAt(
	offset SectionOffset,
) (
	*DebugInfoEntry,
	error,
) {
	entries, err := unit.DebugInfoEntries()
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("invalid debug info entry location (%d)", offset)
	}

	head := entries[0]
	if offset < head.SectionOffset {
		return nil, fmt.Errorf("invalid debug info entry location (%d)", offset)
	} else if offset == head.SectionOffset {
		return head, nil
	}

	entries = entries[1:]
	if len(entries) == 0 {
		return nil, fmt.Errorf("invalid debug info entry location (%d)", offset)
	}

	tail := entries[len(entries)-1]
	if tail.SectionOffset < offset {
		return nil, fmt.Errorf("invalid debug info entry location (%d)", offset)
	} else if offset == tail.SectionOffset {
		return tail, nil
	}

	// Bisection to narrow range, then iterate
	entries = entries[:len(entries)-1]
	for len(entries) > 2 {
		midIdx := len(entries) / 2

		mid := entries[midIdx]
		if offset < mid.SectionOffset {
			entries = entries[:midIdx]
		} else if offset == mid.SectionOffset {
			return mid, nil
		} else {
			entries = entries[midIdx+1:]
		}
	}

	for _, entry := range entries {
		if offset == entry.SectionOffset {
			return entry, nil
		}
	}

	return nil, fmt.Errorf("invalid debug info entry location (%d)", offset)
}

func (unit *CompileUnit) ForEach(process ProcessFunc) error {
	err := unit.maybeParseDebugInfoEntries()
	if err != nil {
		return err
	}

	for _, entry := range unit.entries {
		err := process(entry)
		if err != nil {
			return err
		}
	}

	return nil
}

func (unit *CompileUnit) Visit(enter ProcessFunc, exit ProcessFunc) error {
	root, err := unit.Root()
	if err != nil {
		return err
	}

	return root.Visit(enter, exit)
}

func (unit *CompileUnit) maybeParseDebugInfoEntries() error {
	if unit.root != nil {
		return nil
	}

	abbrevTable, ok := unit.AbbreviationTables[unit.AbbreviationIndex]
	if !ok {
		return fmt.Errorf(
			"failed to parse DIEs. abbreviation table (%d) not found",
			unit.AbbreviationIndex)
	}

	var root *DebugInfoEntry
	entries := []*DebugInfoEntry{}
	scope := []*DebugInfoEntry{}

	decode := NewCursor(unit.ByteOrder(), unit.Content)
	for !decode.HasReachedEnd() {
		code, entry, err := parseDebugInfoEntry(unit, abbrevTable, decode)
		if err != nil {
			return err
		}

		if code == 0 { // end of scope
			if len(scope) == 0 {
				return fmt.Errorf("failed to parse DIEs. too many null DIEs")
			}

			scope = scope[:len(scope)-1]
			continue
		}

		entries = append(entries, entry)

		if root == nil {
			root = entry
		} else if len(scope) > 0 {
			parent := scope[len(scope)-1]
			parent.Children = append(parent.Children, entry)
		} else {
			return fmt.Errorf("failed to parse DIEs. DIE not rooted")
		}

		if entry.HasChildren {
			scope = append(scope, entry)
		}
	}

	if len(scope) != 0 {
		return fmt.Errorf("failed to parse DIES. not enough null DIEs")
	}

	unit.root = root
	unit.entries = entries
	return nil
}

type InformationSection struct {
	*File

	CompileUnits []*CompileUnit
}

func NewInformationSection(file *elf.File) (*InformationSection, error) {
	section := file.GetSection(ElfDebugInformationSection)
	if section == nil {
		return nil, fmt.Errorf("elf .debug_info section not found")
	}

	content, err := section.RawContent()
	if err != nil {
		return nil, fmt.Errorf("failed to read .debug_info section: %w", err)
	}

	units := []*CompileUnit{}

	decode := NewCursor(file.ByteOrder(), content)
	for !decode.HasReachedEnd() {
		unit, err := parseCompileUnit(decode)
		if err != nil {
			return nil, err
		}

		units = append(units, unit)
	}

	return &InformationSection{
		CompileUnits: units,
	}, nil
}

func (section *InformationSection) SetParent(file *File) {
	section.File = file
	for _, unit := range section.CompileUnits {
		unit.File = file
	}
}

func (section *InformationSection) EntryAt(
	offset SectionOffset,
) (
	*DebugInfoEntry,
	error,
) {
	for _, unit := range section.CompileUnits {
		if unit.Contains(offset) {
			return unit.EntryAt(offset)
		}
	}

	return nil, fmt.Errorf("invalid debug info entry location (%d)", offset)
}

func (section *InformationSection) ForEach(process ProcessFunc) error {
	for _, unit := range section.CompileUnits {
		err := unit.ForEach(process)
		if err != nil {
			return err
		}
	}
	return nil
}

func (section *InformationSection) Visit(
	enter ProcessFunc,
	exit ProcessFunc,
) error {
	for _, unit := range section.CompileUnits {
		err := unit.Visit(enter, exit)
		if err != nil {
			return err
		}
	}
	return nil
}

func (section *InformationSection) CompileUnitContainingAddress(
	address elf.FileAddress,
) (
	*CompileUnit,
	error,
) {
	for _, unit := range section.CompileUnits {
		root, err := unit.Root()
		if err != nil {
			return nil, err
		}

		ok, err := root.ContainsAddress(address)
		if err != nil {
			return nil, err
		}

		if ok {
			return unit, nil
		}
	}

	return nil, nil
}

func (section *InformationSection) FunctionEntryContainingAddress(
	address elf.FileAddress,
) (
	*DebugInfoEntry,
	error,
) {
	var result *DebugInfoEntry

	earlyExitErr := fmt.Errorf("early exit")
	retErr := section.ForEach(
		func(entry *DebugInfoEntry) error {
			if entry.Tag != DW_TAG_subprogram {
				return nil
			}

			ok, err := entry.ContainsAddress(address)
			if err != nil {
				return err
			}

			if ok {
				result = entry
				return earlyExitErr
			}

			return nil
		})

	if retErr == earlyExitErr {
		return result, nil
	}

	if retErr != nil {
		return nil, retErr
	}

	return nil, nil
}

func (section *InformationSection) FunctionEntriesWithName(
	name string,
) (
	[]*DebugInfoEntry,
	error,
) {
	result := []*DebugInfoEntry{}
	retErr := section.ForEach(
		func(entry *DebugInfoEntry) error {
			if entry.Tag != DW_TAG_subprogram {
				return nil
			}

			entryName, ok, err := entry.Name()
			if err != nil {
				return err
			}

			if ok && name == entryName {
				result = append(result, entry)
			}

			return nil
		})

	if retErr != nil {
		return nil, retErr
	}

	return result, nil
}
