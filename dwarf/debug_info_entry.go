package dwarf

import (
	"errors"
	"fmt"

	"github.com/pattyshack/bad/elf"
)

// Reference attribute value
type DebugInfoEntryReference struct {
	*File
	SectionOffset
}

func (ref DebugInfoEntryReference) String() string {
	return fmt.Sprintf("DIE@%08x", ref.SectionOffset)
}

func newDebugInfoEntryReference(
	file *File,
	offset SectionOffset,
) *DebugInfoEntryReference {
	return &DebugInfoEntryReference{
		File:          file,
		SectionOffset: offset,
	}
}

func (ref DebugInfoEntryReference) Get() (*DebugInfoEntry, error) {
	entry, err := ref.File.EntryAt(ref.SectionOffset)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get referenced entry (%d): %w",
			ref.SectionOffset,
			err)
	}
	return entry, nil
}

type DebugInfoEntry struct {
	*CompileUnit
	SectionOffset

	*Abbreviation
	Values []interface{}

	Children []*DebugInfoEntry
}

func parseDebugInfoEntry(
	unit *CompileUnit,
	abbrevTable AbbreviationTable,
	decode *Cursor,
) (
	uint64,
	*DebugInfoEntry,
	error,
) {
	startAddr := unit.ContentStart + SectionOffset(decode.Position)

	code, err := decode.ULEB128(64)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse DIE. invalid code: %w", err)
	}

	if code == 0 {
		return 0, nil, nil
	}

	abbrev, ok := abbrevTable[code]
	if !ok {
		return 0, nil, fmt.Errorf(
			"failed to parse DIE. abbreviation (%d) not found",
			code)
	}

	values := make([]interface{}, 0, len(abbrev.AttributeSpecs))
	for _, spec := range abbrev.AttributeSpecs {
		value, err := decode.Value(unit, spec.Format)
		if err != nil {
			return 0, nil, err
		}
		values = append(values, value)
	}

	entry := &DebugInfoEntry{
		CompileUnit:   unit,
		SectionOffset: startAddr,
		Abbreviation:  abbrev,
		Values:        values,
	}

	return code, entry, nil
}

func (entry *DebugInfoEntry) SpecIndex(attr Attribute) int {
	for idx, spec := range entry.AttributeSpecs {
		if attr == spec.Attribute {
			return idx
		}
	}
	return -1
}

func (entry *DebugInfoEntry) Any(attr Attribute) (interface{}, bool) {
	idx := entry.SpecIndex(attr)
	if idx == -1 {
		return nil, false
	}
	return entry.Values[idx], true
}

func (entry *DebugInfoEntry) Address(
	attr Attribute,
) (
	elf.FileAddress,
	bool,
) {
	val, ok := entry.Any(attr)
	if !ok {
		return 0, false
	}
	return val.(elf.FileAddress), true
}

func (entry *DebugInfoEntry) Offset(attr Attribute) (SectionOffset, bool) {
	val, ok := entry.Any(attr)
	if !ok {
		return 0, false
	}
	return val.(SectionOffset), true
}

func (entry *DebugInfoEntry) Bool(attr Attribute) (bool, bool) {
	val, ok := entry.Any(attr)
	if !ok {
		return false, false
	}
	return val.(bool), true
}

func (entry *DebugInfoEntry) Uint(attr Attribute) (uint64, bool) {
	val, ok := entry.Any(attr)
	if !ok {
		return 0, false
	}
	return val.(uint64), true
}

func (entry *DebugInfoEntry) Int(attr Attribute) (int64, bool) {
	val, ok := entry.Any(attr)
	if !ok {
		return 0, false
	}
	return val.(int64), true
}

func (entry *DebugInfoEntry) Bytes(attr Attribute) ([]byte, bool) {
	val, ok := entry.Any(attr)
	if !ok {
		return nil, false
	}
	return val.([]byte), true
}

func (entry *DebugInfoEntry) String(attr Attribute) (string, bool) {
	val, ok := entry.Any(attr)
	if !ok {
		return "", false
	}
	return val.(string), true
}

func (entry *DebugInfoEntry) Reference(
	attr Attribute,
) (
	*DebugInfoEntryReference,
	bool,
) {
	val, ok := entry.Any(attr)
	if !ok {
		return nil, false
	}
	return val.(*DebugInfoEntryReference), true
}

func (entry *DebugInfoEntry) EvaluateLocation(
	attr Attribute,
	context ExpressionContext,
	inFrameInfo bool,
	initializeStackWithCFA bool,
) (
	Location,
	error,
) {
	idx := entry.SpecIndex(attr)
	if idx == -1 {
		return nil, nil
	}

	value := entry.Values[idx]

	switch entry.AttributeSpecs[idx].Format {
	case DW_FORM_exprloc:
		return EvaluateExpression(context, inFrameInfo, value.([]byte), false)
	case DW_FORM_sec_offset: // in .debug_loc
		root, err := entry.CompileUnit.Root()
		if err != nil {
			return nil, err
		}

		addressRanges, err := root.AddressRanges()
		if err != nil {
			return nil, err
		}
		if len(addressRanges) == 0 {
			return nil, fmt.Errorf("compile unit has invalid address ranges")
		}

		return entry.CompileUnit.File.LocationSection.EvaluateLocation(
			value.(SectionOffset),
			addressRanges[0].Low,
			context,
			inFrameInfo)
	default:
		return nil, fmt.Errorf("invalid location type")
	}
}

func (entry *DebugInfoEntry) Name() (
	string,
	bool, // false if not found
	error,
) {
	refIdx := -1
	for idx, spec := range entry.AttributeSpecs {
		if spec.Attribute == DW_AT_name {
			return entry.Values[idx].(string), true, nil
		} else if spec.Attribute == DW_AT_specification {
			// Current entry is a function declaration. The real definition is in the
			// referenced entry.
			refIdx = idx
		} else if spec.Attribute == DW_AT_abstract_origin {
			// Current entry is an inlined function, the referenced entry is the
			// original function definition.
			refIdx = idx
		}
	}

	if refIdx == -1 {
		return "", false, nil
	}

	ref := entry.Values[refIdx].(*DebugInfoEntryReference)
	refEntry, err := ref.Get()
	if err != nil {
		return "", false, err
	}

	return refEntry.Name()
}

func (entry *DebugInfoEntry) TypeEntry() (*DebugInfoEntry, error) {
	defEntry := entry

	ref, ok := entry.Reference(DW_AT_abstract_origin)
	if ok {
		// Current entry is an inlined function variable, the referenced entry is
		// the original function variable definition.
		var err error
		defEntry, err = ref.Get()
		if err != nil {
			return nil, fmt.Errorf("cannot get inlined variable definition: %w", err)
		}
	}

	ref, ok = defEntry.Reference(DW_AT_type)
	if !ok {
		return nil, fmt.Errorf("type entry not found")
	}

	return ref.Get()
}

func (entry *DebugInfoEntry) FileEntry() (*FileEntry, error) {
	var idx uint64
	var ok bool
	if entry.Tag == DW_TAG_inlined_subroutine {
		idx, ok = entry.Uint(DW_AT_call_file)
	} else {
		idx, ok = entry.Uint(DW_AT_decl_file)
	}

	if !ok {
		return nil, nil
	}

	if entry.lineTable == nil {
		return nil, fmt.Errorf("compile unit has no line table")
	}

	if idx == 0 || idx-1 >= uint64(len(entry.lineTable.FileEntries)) {
		return nil, fmt.Errorf("out of bound line table file index")
	}

	return entry.lineTable.FileEntries[idx-1], nil
}

func (entry *DebugInfoEntry) Line() (int64, bool) {
	if entry.Tag == DW_TAG_inlined_subroutine {
		val, ok := entry.Uint(DW_AT_call_line)
		return int64(val), ok
	}

	val, ok := entry.Uint(DW_AT_decl_line)
	return int64(val), ok
}

func (entry *DebugInfoEntry) AddressRanges() (AddressRanges, error) {
	lowAddr, lowOk := entry.Address(DW_AT_low_pc)
	high, highOk := entry.Any(DW_AT_high_pc)

	if lowOk && highOk {
		switch val := high.(type) {
		case elf.FileAddress:
			return AddressRanges{
				{
					Low:  lowAddr,
					High: val,
				},
			}, nil
		case uint64:
			return AddressRanges{
				{
					Low:  lowAddr,
					High: lowAddr + elf.FileAddress(val),
				},
			}, nil
		default:
			panic("should never happen")
		}
	}

	index, indexOk := entry.Offset(DW_AT_ranges)

	if !indexOk {
		return nil, nil
	}

	return entry.AddressRangesAt(index, lowAddr)
}

func (entry *DebugInfoEntry) ContainsAddress(
	address elf.FileAddress,
) (
	bool,
	error,
) {
	addressRanges, err := entry.AddressRanges()
	if err != nil {
		return false, err
	}

	return addressRanges.Contains(address), nil
}

// NOTE: Finding the method definition from its declaration is extremely
// awkward. The declaration has no attribute information on where to locate
// the definition. Furthermore, inlined method creates another indirection.
// We need to handle the following cases:
//
//  1. declaration = definition
//  2. non-inlined method:
//     decl <-(DW_AT_specification)- def
//  3. inlined method:
//     decl <-(DW_AT_specification)- partial def <-(DW_AT_abstract_origin)- def
func (methodDeclaration *DebugInfoEntry) FindMethodDefinitionEntry() (
	*DebugInfoEntry,
	error,
) {
	indirections := map[*DebugInfoEntry]*DebugInfoEntry{}
	err := methodDeclaration.InformationSection.ForEach(
		func(entry *DebugInfoEntry) error {
			if entry.Tag != DW_TAG_subprogram &&
				entry.Tag != DW_TAG_inlined_subroutine {
				return nil
			}

			ref, ok := entry.Reference(DW_AT_specification)
			if ok {
				linked, refErr := ref.Get()
				if refErr != nil {
					return refErr
				}

				indirections[linked] = entry
			}

			ref, ok = entry.Reference(DW_AT_abstract_origin)
			if ok {
				linked, refErr := ref.Get()
				if refErr != nil {
					return refErr
				}

				indirections[linked] = entry
			}

			return nil
		})
	if err != nil {
		return nil, err
	}

	current := methodDeclaration
	for current != nil {
		addressRanges, err := current.AddressRanges()
		if err != nil {
			return nil, err
		}

		if len(addressRanges) != 0 {
			return current, nil
		}

		current = indirections[current]
	}

	return nil, fmt.Errorf("method definition not found")
}

func (entry *DebugInfoEntry) Visit(enter ProcessFunc, exit ProcessFunc) error {
	skipVisitingChildren := false
	if enter != nil {
		err := enter(entry)
		if err != nil {
			if errors.Is(err, ErrSkipVisitingChildren) {
				skipVisitingChildren = true
			} else {
				return err
			}
		}
	}

	if !skipVisitingChildren {
		for _, child := range entry.Children {
			err := child.Visit(enter, exit)
			if err != nil {
				return err
			}
		}
	}

	if exit != nil {
		err := exit(entry)
		if err != nil {
			return err
		}
	}

	return nil
}
