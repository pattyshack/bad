package loadedelves

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/dwarf"
	"github.com/pattyshack/bad/elf"
	"github.com/pattyshack/bad/procfs"
)

const (
	VDSOFileName = "linux-vdso.so.1"

	symbolTableName        = ".symtab"
	dynamicSymbolTableName = ".dynsym"
)

type File struct {
	*elf.File
	LoadBias uint64

	Dwarf *dwarf.File // optional

	symbolTables []*elf.SymbolTableSection
}

func newExecutableFile(pid int) (*File, error) {
	content, err := os.ReadFile(procfs.GetExecutableSymlinkPath(pid))
	if err != nil {
		return nil, fmt.Errorf("failed to read executable elf file: %w", err)
	}

	file, err := newFile("", content, 0)
	if err != nil {
		return nil, err
	}

	aux, err := procfs.GetAuxiliaryVector(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to compute elf load bias: %w", err)
	}

	loadedEntryPointAddress, ok := aux[procfs.AT_Entry]
	if !ok {
		return nil, fmt.Errorf(
			"failed to compute elf load bias. loaded entry point address not found.")
	}

	file.FileName = ""
	file.LoadBias = loadedEntryPointAddress - file.EntryPointAddress
	return file, nil
}

func newDynamicallyLoadedFile(
	path string,
	address VirtualAddress,
) (
	*File,
	error,
) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read elf file (%s): %w", path, err)
	}

	return newFile(path, content, uint64(address))
}

func newVDSOFile(
	mem *memory.VirtualMemory,
	address VirtualAddress,
) (
	*File,
	error,
) {
	headerBytes := make([]byte, elf.Elf64HeaderSize)

	n, err := mem.Read(address, headerBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read vDSO elf header: %w", err)
	}
	if n != elf.Elf64HeaderSize {
		return nil, fmt.Errorf(
			"incorrect number of vDSO elf header bytes read (%d)",
			n)
	}

	header := &elf.ElfHeader{}
	n, err = binary.Decode(headerBytes, binary.LittleEndian, header)
	if err != nil {
		return nil, fmt.Errorf("failed to decode vDSO elf header: %w", err)
	}
	if n != elf.Elf64HeaderSize {
		panic("should never happen")
	}

	// NOTE: We're assuming that vDSO's elf section headers are at the end of
	// the file, which isn't generally true for elf files.
	size := int(header.SectionHeaderOffset) +
		int(header.SectionHeaderEntrySize)*int(header.NumSectionHeaderEntries)

	content := make([]byte, size)

	n, err = mem.Read(address, content)
	if err != nil {
		return nil, fmt.Errorf("failed to read vDSO elf content: %w", err)
	}
	if n != size {
		return nil, fmt.Errorf(
			"incorrect number of vDSO elf content bytes read (%d)",
			n)
	}

	return newFile(VDSOFileName, content, uint64(address))
}

func newFile(path string, content []byte, loadBias uint64) (*File, error) {
	elfFile, err := elf.ParseBytes(path, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse elf (%s): %w", path, err)
	}

	dwarfFile, err := dwarf.NewFile(elfFile)
	if err != nil {
		if !errors.Is(err, dwarf.ErrSectionNotFound) {
			return nil, fmt.Errorf("failed to parse dwarf (%s): %w", path, err)
		}
		dwarfFile = nil
	}

	symbolTables := []*elf.SymbolTableSection{}

	section := elfFile.GetSection(symbolTableName)
	if section != nil {
		symbolTables = append(symbolTables, section.(*elf.SymbolTableSection))
	}

	section = elfFile.GetSection(dynamicSymbolTableName)
	if section != nil {
		symbolTables = append(symbolTables, section.(*elf.SymbolTableSection))
	}

	return &File{
		File:         elfFile,
		Dwarf:        dwarfFile,
		LoadBias:     loadBias,
		symbolTables: symbolTables,
	}, nil
}

func (file *File) ToFileAddress(
	address VirtualAddress,
) elf.FileAddress {
	return elf.FileAddress(uint64(address) - file.LoadBias)
}

func (file *File) ToVirtualAddress(
	address elf.FileAddress,
) VirtualAddress {
	return VirtualAddress(uint64(address) + file.LoadBias)
}

func (file *File) EntryPointFileAddress() elf.FileAddress {
	return elf.FileAddress(file.EntryPointAddress)
}

func (file *File) SymbolsByName(name string) []*elf.Symbol {
	results := []*elf.Symbol{}
	for _, table := range file.symbolTables {
		results = append(results, table.SymbolsByName(name)...)
	}

	return results
}

func (file *File) SymbolAt(address VirtualAddress) *elf.Symbol {
	fileAddr := file.ToFileAddress(address)

	for _, table := range file.symbolTables {
		symbol := table.SymbolAt(fileAddr)
		if symbol != nil {
			return symbol
		}
	}

	return nil
}

func (file *File) SymbolSpans(address VirtualAddress) *elf.Symbol {
	fileAddr := file.ToFileAddress(address)

	for _, table := range file.symbolTables {
		symbol := table.SymbolSpans(fileAddr)
		if symbol != nil {
			return symbol
		}
	}

	return nil
}

func (file *File) FunctionEntryContainingAddress(
	address VirtualAddress,
) (
	*dwarf.DebugInfoEntry,
	error,
) {
	if file.Dwarf == nil {
		return nil, nil
	}

	return file.Dwarf.FunctionEntryContainingAddress(file.ToFileAddress(address))
}

func (file *File) FunctionEntriesWithName(
	name string,
) (
	[]*dwarf.DebugInfoEntry,
	error,
) {
	if file.Dwarf == nil {
		return nil, nil
	}

	return file.Dwarf.FunctionEntriesWithName(name)
}

func (file *File) LineEntryAt(
	address VirtualAddress,
) (
	*dwarf.LineEntry,
	error,
) {
	if file.Dwarf == nil {
		return nil, nil
	}

	return file.Dwarf.GetLineEntryByAddress(file.ToFileAddress(address))
}

func (file *File) LineEntriesByLine(
	pathName string,
	line int,
) (
	[]*dwarf.LineEntry,
	error,
) {
	if file.Dwarf == nil {
		return nil, nil
	}

	return file.Dwarf.GetLineEntriesByLine(pathName, int64(line))
}

func (file *File) ComputeUnwindRulesAt(
	pc VirtualAddress,
) (
	*dwarf.UnwindRules,
	error,
) {
	if file.Dwarf == nil {
		return nil, nil
	}

	return file.Dwarf.ComputeUnwindRulesAt(file.ToFileAddress(pc))
}
