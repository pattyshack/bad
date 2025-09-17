package elf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Resources:
// https://refspecs.linuxfoundation.org/

type machineSpec struct {
	MachineArchitecture
	DataEncoding
	OperatingSystemABI
}

var (
	// NOTE: For now, only supports linux system v abi
	supportedArchitecture = map[MachineArchitecture]machineSpec{
		MachineArchitectureX86_64: machineSpec{
			MachineArchitecture: MachineArchitectureX86_64,
			DataEncoding:        DataEncodingTwosComplementLittleEndian,
			OperatingSystemABI:  OperatingSystemABIUnixSystemV,
		},
	}
)

type File struct {
	ElfHeader
	Sections       []Section
	ProgramHeaders []ProgramHeaderEntry
}

func (file *File) GetSection(name string) (Section, bool) {
	for _, section := range file.Sections {
		if section.Name() == name {
			return section, true
		}
	}

	return nil, false
}

type parser struct {
	content []byte

	binary.ByteOrder

	File
}

func Parse(reader io.Reader) (*File, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read elf file: %w", err)
	}

	return ParseBytes(content)
}

func ParseBytes(content []byte) (*File, error) {
	p := parser{
		content: content,
	}

	err := p.parse()
	if err != nil {
		return nil, err
	}

	return &p.File, nil
}

func (p *parser) parse() error {
	// NOTE: identifier (e_ident) has no endian-ness.  We must parse identifier
	// to determine the elf file's endian-ness (including the elf header).
	err := p.parseIdentifier()
	if err != nil {
		return err
	}

	err = p.parseHeader()
	if err != nil {
		return err
	}

	err = p.parseSectionHeaders()
	if err != nil {
		return err
	}

	err = p.parseProgramHeaders()
	if err != nil {
		return err
	}

	return nil
}

func (p *parser) parseIdentifier() error {
	id := &Identifier{}

	n, err := binary.Decode(p.content, binary.NativeEndian, id)
	if err != nil {
		return fmt.Errorf("failed to parse identifier: %w", err)
	}

	if n != ElfIdentifierSize {
		panic("should never happen")
	}

	if !bytes.Equal(id.Magic[:], IdentifierMagic) {
		return fmt.Errorf("invalid elf magic number")
	}

	if id.Class != Class64 {
		return fmt.Errorf("unsupported elf class: %s", id.Class)
	}

	switch id.DataEncoding {
	case DataEncodingTwosComplementLittleEndian:
		p.ByteOrder = binary.LittleEndian
	case DataEncodingTwosComplementBigEndian:
		p.ByteOrder = binary.BigEndian
	default:
		return fmt.Errorf("unsupported data encoding: %s", id.DataEncoding)
	}

	if id.IdentifierVersion != IdentifierVersion {
		return fmt.Errorf(
			"unsupported identifier version: %d",
			id.IdentifierVersion)
	}

	if id.OperatingSystemABI != OperatingSystemABIUnixSystemV {
		return fmt.Errorf("unsupported os/abi: %s", id.OperatingSystemABI)
	}

	if id.ABIVersion != ABIVersion {
		return fmt.Errorf("unsupported abi verison: %d", id.ABIVersion)
	}

	for _, padding := range id.Padding {
		if padding != 0 {
			return fmt.Errorf("invalid identifier padding")
		}
	}

	return nil
}

func (p *parser) parseHeader() error {
	n, err := binary.Decode(p.content, p.ByteOrder, &p.ElfHeader)
	if err != nil {
		return fmt.Errorf("failed to parse header: %w", err)
	}

	if n != Elf64HeaderSize {
		panic("should never happen")
	}

	spec, ok := supportedArchitecture[p.MachineArchitecture]
	if !ok {
		return fmt.Errorf(
			"unsupported machine architecture: %s",
			p.MachineArchitecture)
	}

	if spec.DataEncoding != p.DataEncoding {
		return fmt.Errorf(
			"invalid data encoding (%s) for machine architecture (%s)",
			p.DataEncoding,
			p.MachineArchitecture)
	}

	if spec.OperatingSystemABI != p.OperatingSystemABI {
		return fmt.Errorf(
			"invalid os/abi (%s) for machine architecture (%s)",
			p.OperatingSystemABI,
			p.MachineArchitecture)
	}

	if p.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported format version: %d", p.FormatVersion)
	}

	if p.ArchitectureFlags != 0 {
		return fmt.Errorf("unexpected architecture flags: %x", p.ArchitectureFlags)
	}

	if p.ElfHeaderSize != Elf64HeaderSize {
		return fmt.Errorf("unexpected elf64 header size: %d", p.ElfHeaderSize)
	}

	if p.ProgramHeaderEntrySize != Elf64ProgramHeaderEntrySize {
		return fmt.Errorf(
			"unexpected elf64 program header entry size: %d",
			p.ProgramHeaderEntrySize)
	}

	if p.SectionHeaderEntrySize != Elf64SectionHeaderEntrySize {
		return fmt.Errorf(
			"unexpected elf64 section header entry size: %d",
			p.SectionHeaderEntrySize)
	}

	// For simplicity, we'll disallow extended section header.  Most elf structs
	// (e.g., Elf64_Sym.st_shndx) don't support extended section indexing.
	//
	// https://docs.oracle.com/en/operating-systems/solaris/oracle-solaris/11.4/linkers-libraries/extended-section-header.html
	if p.SectionHeaderOffset > 0 && p.NumSectionHeaderEntries == 0 {
		return fmt.Errorf("extended section header not supported")
	}

	return nil
}

func (p *parser) parseSectionHeaders() error {
	if p.NumSectionHeaderEntries == 0 {
		return nil
	}

	if p.SectionHeaderOffset >= uint64(len(p.content)) {
		return fmt.Errorf(
			"out of bound section header offset (%d)",
			p.SectionHeaderOffset)
	}

	sectionHeaders := make([]SectionHeaderEntry, p.NumSectionHeaderEntries)
	n, err := binary.Decode(
		p.content[p.SectionHeaderOffset:],
		p.ByteOrder,
		sectionHeaders)
	if err != nil {
		return fmt.Errorf("failed to read section header entries: %w", err)
	}
	if n != int(p.NumSectionHeaderEntries)*Elf64SectionHeaderEntrySize {
		panic("should never happen")
	}

	for _, header := range sectionHeaders {
		var sectionContent []byte
		if header.SectionType != SectionTypeNoSpace {
			start := header.Offset
			end := start + header.Size
			if end > uint64(len(p.content)) {
				return fmt.Errorf("out of bound section (%d > %d)", end, len(p.content))
			}

			sectionContent = p.content[start:end]
		}

		// TODO Relocations
		switch header.SectionType {
		case SectionTypeStringTable:
			p.Sections = append(
				p.Sections,
				NewStringTableSection(header, sectionContent))
		case SectionTypeSymbolTable,
			SectionTypeDynamicSymbolTable:

			table, err := p.parseSymbolTable(header, sectionContent)
			if err != nil {
				return err
			}
			p.Sections = append(p.Sections, table)
		case SectionTypeNote:
			note, err := p.parseNote(header, sectionContent)
			if err != nil {
				return err
			}
			p.Sections = append(p.Sections, note)
		default:
			p.Sections = append(p.Sections, newRawSection(header, sectionContent))
		}
	}

	// Bind section names
	if p.SectionStringTableIndex != SectionIndexUndefined {
		idx := int(p.SectionStringTableIndex)
		if idx > len(p.Sections) {
			return fmt.Errorf(
				"section name index out of bound (%d > %d)",
				idx,
				len(p.Sections))
		}

		table, ok := p.Sections[idx].(*StringTableSection)
		if !ok {
			return fmt.Errorf("section name index does not point to a string table")
		}

		for _, section := range p.Sections {
			section.BindSectionNameTable(table)
		}
	}

	// Bind sh_link section
	// See elf spec. Figure 1-12. sh_link and sh_info Interpretation.
	for _, section := range p.Sections {
		hdr := section.Header()

		if hdr.Link == 0 { // section 0 is always undefined
			continue
		}

		switch hdr.SectionType {
		case SectionTypeDynamic,
			SectionTypeSymbolTable,
			SectionTypeDynamicSymbolTable:
			if hdr.Link > uint32(len(p.Sections)) {
				return fmt.Errorf(
					"string table index out of bound (%d > %d)",
					hdr.Link,
					len(p.Sections))
			}

			table, ok := p.Sections[hdr.Link].(*StringTableSection)
			if !ok {
				return fmt.Errorf("string table index does not point to a string table")
			}

			section.BindStringTable(table)
		case SectionTypeSymbolHashTable,
			SectionTypeRelocationWithAddends,
			SectionTypeRelocationNoAddends:

			if hdr.Link > uint32(len(p.Sections)) {
				return fmt.Errorf(
					"symbol table index out of bound (%d > %d)",
					hdr.Link,
					len(p.Sections))
			}

			table, ok := p.Sections[hdr.Link].(*SymbolTableSection)
			if !ok {
				return fmt.Errorf(
					"symbol table index (%d) does not point to a symbol table (%s)",
					hdr.Link,
					p.Sections[hdr.Link].Name())
			}

			section.BindSymbolTable(table)
		}
	}

	// Bind sh_info section
	for _, section := range p.Sections {
		hdr := section.Header()

		if hdr.Info == 0 { // section 0 is always undefined
			continue
		}

		switch hdr.SectionType {
		case SectionTypeRelocationWithAddends, SectionTypeRelocationNoAddends:
			if hdr.Info > uint32(len(p.Sections)) {
				return fmt.Errorf(
					"relocations index out of bound (%d > %d)",
					hdr.Info,
					len(p.Sections))
			}

			// TODO relocations type
			relocations, ok := p.Sections[hdr.Info].(*RawSection)
			if !ok {
				return fmt.Errorf("relocations index does not point to relocations")
			}

			section.BindRelocations(relocations)
		}
	}

	return nil
}

func (p *parser) parseSymbolTable(
	header SectionHeaderEntry,
	content []byte,
) (
	*SymbolTableSection,
	error,
) {
	if len(content)%Elf64SymbolEntrySize != 0 {
		return nil, fmt.Errorf("invalid symbol table size (%d)", len(content))
	}

	numEntries := len(content) / Elf64SymbolEntrySize
	rawEntries := make([]SymbolEntry, numEntries)
	n, err := binary.Decode(content, p.ByteOrder, rawEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to parse symbol table: %w", err)
	}
	if n != len(content) {
		panic("should never happen")
	}

	table := &SymbolTableSection{
		BaseSection: newBaseSection(header),
	}

	symbols := make([]*Symbol, 0, numEntries)
	for _, entry := range rawEntries {
		symbols = append(
			symbols,
			&Symbol{
				SymbolEntry: entry,
				Parent:      table,
			})
	}

	table.Symbols = symbols
	return table, nil
}

func (p *parser) parseProgramHeaders() error {
	if p.NumProgramHeaderEntries == 0 {
		return nil
	}

	if p.ProgramHeaderOffset >= uint64(len(p.content)) {
		return fmt.Errorf(
			"out of bound program header offset (%d)",
			p.ProgramHeaderOffset)
	}

	programHeaders := make([]ProgramHeaderEntry, p.NumProgramHeaderEntries)
	n, err := binary.Decode(
		p.content[p.ProgramHeaderOffset:],
		p.ByteOrder,
		programHeaders)
	if err != nil {
		return fmt.Errorf("failed to read program header entries: %w", err)
	}
	if n != int(p.NumProgramHeaderEntries)*Elf64ProgramHeaderEntrySize {
		panic("should never happen")
	}

	p.ProgramHeaders = programHeaders
	return nil
}

func (p *parser) parseNote(
	header SectionHeaderEntry,
	content []byte,
) (
	*NoteSection,
	error,
) {
	entries := []NoteEntry{}

	// NOTE: even though Elf64_Nhdr is defined, it looks like tools continue to
	// use Elf32_Nhdr / 4-byte aligned note entries.
	for len(content) > 0 {
		if len(content)%4 != 0 {
			return nil, fmt.Errorf("failed to parse note section. not 4-byte aligned")
		}

		noteHdr := &NoteHeader{}
		n, err := binary.Decode(content, p.ByteOrder, noteHdr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse note header: %w", err)
		}
		if n != NoteHeaderSize {
			panic("should never happen")
		}
		content = content[n:]

		if len(content) < int(noteHdr.NameSize) {
			return nil, fmt.Errorf(
				"failed to parse note entry. not enough name bytes")
		}
		if len(content)%4 != 0 {
			return nil, fmt.Errorf("failed to parse note entry. not 4-byte aligned")
		}

		name := string(content[:noteHdr.NameSize])

		// make descStart 4 byte aligned.
		descStart := ((noteHdr.NameSize + 3) / 4) * 4

		content = content[descStart:]

		if len(content) < int(noteHdr.DescriptionSize) {
			return nil, fmt.Errorf(
				"failed to parse note entry. not enough description bytes")
		}
		if len(content)%4 != 0 {
			return nil, fmt.Errorf("failed to parse note entry. not 4-byte aligned")
		}

		desc := string(content[:noteHdr.DescriptionSize])

		entries = append(
			entries,
			NoteEntry{
				Name:        name,
				Description: desc,
				Type:        noteHdr.Type,
			})

		// make nextEntryStart 4 byte aligned.
		nextEntryStart := ((noteHdr.DescriptionSize + 3) / 4) * 4
		content = content[nextEntryStart:]
	}

	return newNoteSection(header, entries), nil
}
