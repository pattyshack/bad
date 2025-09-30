package dwarf

import (
	"fmt"

	"github.com/pattyshack/bad/elf"
)

type AttributeSpec struct {
	Attribute
	Format
}

type Abbreviation struct {
	Code uint64
	Tag
	HasChildren    bool
	AttributeSpecs []AttributeSpec
}

type AbbreviationTable map[uint64]*Abbreviation

type AbbreviationSection struct {
	AbbreviationTables map[SectionOffset]AbbreviationTable
}

func NewAbbreviationSection(file *elf.File) (*AbbreviationSection, error) {
	section := file.GetSection(ElfDebugAbbreviationSection)
	if section == nil {
		return nil, fmt.Errorf("elf .debug_abbrev %w", ErrSectionNotFound)
	}

	content, err := section.RawContent()
	if err != nil {
		return nil, fmt.Errorf("failed to read elf .debug_abbrev section: %w", err)
	}

	tables := map[SectionOffset]AbbreviationTable{}

	decode := NewCursor(file.ByteOrder(), content)
	for !decode.HasReachedEnd() {
		tableId := SectionOffset(decode.Position)
		table := AbbreviationTable{}

		for {
			code, err := decode.ULEB128(64)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to parse abbreviation. invalid code: %w",
					err)
			}

			if code == 0 {
				break
			}

			tag, err := decode.ULEB128(64)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to parse abbreviation. invalid tag: %w",
					err)
			}

			hasChildren, err := decode.U8()
			if err != nil {
				return nil, fmt.Errorf(
					"failed to parse abbreviation. invalid hasChildren: %w",
					err)
			}

			var specs []AttributeSpec
			for {
				attribute, err := decode.ULEB128(64)
				if err != nil {
					return nil, fmt.Errorf(
						"failed to parse abbreviation. invalid attribute: %w",
						err)
				}

				format, err := decode.ULEB128(64)
				if err != nil {
					return nil, fmt.Errorf(
						"failed to parse abbreviation. invalid format: %w",
						err)
				}

				if attribute == 0 {
					break
				}

				specs = append(
					specs,
					AttributeSpec{
						Attribute: Attribute(attribute),
						Format:    Format(format),
					})
			}

			table[code] = &Abbreviation{
				Code:           code,
				Tag:            Tag(tag),
				HasChildren:    hasChildren != 0,
				AttributeSpecs: specs,
			}
		}

		tables[tableId] = table
	}

	return &AbbreviationSection{
		AbbreviationTables: tables,
	}, nil
}
