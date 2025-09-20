package elf

import (
	"bytes"
	"fmt"

	"github.com/ianlancetaylor/demangle"
)

type FileAddress uint64

type Section interface {
	Header() SectionHeaderEntry

	BindSectionNameTable(sectionNames *StringTableSection)
	Name() string

	RawContent() ([]byte, error)

	// See elf spec. Figure 1-12. sh_link and sh_info interpretation.
	// TODO replace RawSection with RelocationSection
	BindStringTable(stringTable *StringTableSection)
	BindSymbolTable(symbolTable *SymbolTableSection)
	BindRelocations(relocations *RawSection)
}

type BaseSection struct {
	SectionHeaderEntry

	sectionNameTable *StringTableSection
	name             string
}

func newBaseSection(header SectionHeaderEntry) BaseSection {
	return BaseSection{
		SectionHeaderEntry: header,
	}
}

func (base *BaseSection) Header() SectionHeaderEntry {
	return base.SectionHeaderEntry
}

func (base *BaseSection) Name() string {
	return base.name
}

func (base *BaseSection) BindSectionNameTable(
	sectionNames *StringTableSection,
) {
	base.sectionNameTable = sectionNames
	base.name = sectionNames.Get(base.NameIndex)
}

func (BaseSection) RawContent() ([]byte, error) {
	return nil, fmt.Errorf("cannot get raw content")
}

func (BaseSection) BindStringTable(table *StringTableSection) {
}

func (BaseSection) BindSymbolTable(table *SymbolTableSection) {
}

func (BaseSection) BindRelocations(relocations *RawSection) {
}

type RawSection struct {
	BaseSection

	Content []byte
}

func newRawSection(header SectionHeaderEntry, buffer []byte) *RawSection {
	content := make([]byte, len(buffer))
	copy(content, buffer)

	return &RawSection{
		BaseSection: newBaseSection(header),
		Content:     content,
	}
}

func (section *RawSection) RawContent() ([]byte, error) {
	return section.Content, nil
}

type StringTableSection struct {
	BaseSection

	Content []byte
}

func NewStringTableSection(
	header SectionHeaderEntry,
	buffer []byte,
) *StringTableSection {
	content := make([]byte, len(buffer))
	copy(content, buffer)

	return &StringTableSection{
		BaseSection: newBaseSection(header),
		Content:     content,
	}
}

func (table *StringTableSection) Get(index uint32) string {
	if index >= uint32(len(table.Content)) {
		return ""
	}

	chunk := table.Content[index:]
	end := bytes.IndexByte(chunk, 0)
	if end == -1 {
		return ""
	}

	return string(chunk[:end])
}

func (table *StringTableSection) NumEntries() int {
	count := 0
	for _, b := range table.Content[1:] {
		if b == 0 {
			count += 1
		}
	}
	return count
}

type Symbol struct {
	SymbolEntry

	Parent        *SymbolTableSection
	Name          string
	DemangledName string // human readable c++ / rust name
}

func (symbol Symbol) PrettyName() string {
	if symbol.DemangledName != "" {
		return symbol.DemangledName
	}

	return symbol.Name
}

func (symbol Symbol) Type() SymbolType {
	return SymbolInfoToType(symbol.Info)
}

func (symbol Symbol) Binding() SymbolBinding {
	return SymbolInfoToBinding(symbol.Info)
}

func (symbol Symbol) AddressRange() (FileAddress, FileAddress, bool) {
	if symbol.Value == 0 ||
		symbol.NameIndex == 0 ||
		symbol.Type() == SymbolTypeTLSObject {

		return 0, 0, false
	}

	start := FileAddress(symbol.Value)
	end := FileAddress(symbol.Value + symbol.Size)
	return start, end, true
}

type SymbolTableSection struct {
	BaseSection

	Symbols []*Symbol

	stringTable *StringTableSection
}

func (table *SymbolTableSection) BindStringTable(names *StringTableSection) {
	table.stringTable = names
	for _, symbol := range table.Symbols {
		symbol.Name = names.Get(symbol.NameIndex)
		val, err := demangle.ToString(symbol.Name)
		if err == nil {
			symbol.DemangledName = val
		}
	}
}

func (table *SymbolTableSection) SymbolsByName(name string) []*Symbol {
	result := []*Symbol{}
	for _, symbol := range table.Symbols {
		if symbol.Name == name || symbol.DemangledName == name {
			result = append(result, symbol)
		}
	}
	return result
}

func (table *SymbolTableSection) SymbolAt(address FileAddress) *Symbol {
	for _, symbol := range table.Symbols {
		low, _, ok := symbol.AddressRange()
		if ok && low == address {
			return symbol
		}
	}

	return nil
}

func (table *SymbolTableSection) SymbolSpans(address FileAddress) *Symbol {
	for _, symbol := range table.Symbols {
		low, high, ok := symbol.AddressRange()
		if ok && low <= address && address < high {
			return symbol
		}
	}

	return nil
}

type NoteEntry struct {
	Name        string // name is usually human readable
	Description string // description has no standard format and may be unreadable
	Type        uint32
}

type NoteSection struct {
	BaseSection

	Entries []NoteEntry
}

func newNoteSection(
	header SectionHeaderEntry,
	entries []NoteEntry,
) *NoteSection {
	return &NoteSection{
		BaseSection: newBaseSection(header),
		Entries:     entries,
	}
}
