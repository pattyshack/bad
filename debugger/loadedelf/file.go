package loadedelf

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/dwarf"
	"github.com/pattyshack/bad/elf"
	"github.com/pattyshack/bad/procfs"
)

const (
	symbolTableName        = ".symtab"
	dynamicSymbolTableName = ".dynsym"
)

type Files struct {
	Files []*File
}

func NewFiles() *Files {
	return &Files{}
}

func (files *Files) ToVirtualAddress(
	elfFile *elf.File,
	fileAddress elf.FileAddress,
) (
	VirtualAddress,
	error,
) {
	for _, file := range files.Files {
		if file.File == elfFile {
			return file.ToVirtualAddress(fileAddress), nil
		}
	}

	return 0, fmt.Errorf(
		"cannot covert file address to virtual address. elf file not loaded")
}

func (files *Files) SymbolToVirtualAddress(
	symbol *elf.Symbol,
) (
	VirtualAddress,
	error,
) {
	return files.ToVirtualAddress(
		symbol.Parent.File(),
		elf.FileAddress(symbol.Value))
}

func (files *Files) LineEntryToVirtualAddress(
	entry *dwarf.LineEntry,
) (
	VirtualAddress,
	error,
) {
	return files.ToVirtualAddress(
		entry.CompileUnit().File.File,
		entry.FileAddress)
}

func (files *Files) ToVirtualAddressRanges(
	die *dwarf.DebugInfoEntry,
) (
	AddressRanges,
	error,
) {
	fars, err := die.AddressRanges()
	if err != nil {
		return nil, err
	}

	result := AddressRanges{}
	for _, far := range fars {
		low, err := files.ToVirtualAddress(die.CompileUnit.File.File, far.Low)
		if err != nil {
			return nil, err
		}

		high, err := files.ToVirtualAddress(die.CompileUnit.File.File, far.High)
		if err != nil {
			return nil, err
		}

		result = append(
			result,
			AddressRange{
				Low:  low,
				High: high,
			})
	}

	return result, nil
}

func (files *Files) LoadBinary(pid int) (*File, error) {
	content, err := os.ReadFile(procfs.GetExecutableSymlinkPath(pid))
	if err != nil {
		return nil, fmt.Errorf("failed to read elf file: %w", err)
	}

	elfFile, err := elf.ParseBytes(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse elf file: %w", err)
	}

	dwarfFile, err := dwarf.NewFile(elfFile)
	if err != nil {
		if !errors.Is(err, dwarf.ErrSectionNotFound) {
			return nil, err
		}
		dwarfFile = nil
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

	loadBias := loadedEntryPointAddress - elfFile.EntryPointAddress

	symbolTables := []*elf.SymbolTableSection{}

	section := elfFile.GetSection(symbolTableName)
	if section != nil {
		symbolTables = append(symbolTables, section.(*elf.SymbolTableSection))
	}

	section = elfFile.GetSection(dynamicSymbolTableName)
	if section != nil {
		symbolTables = append(symbolTables, section.(*elf.SymbolTableSection))
	}

	file := &File{
		File:         elfFile,
		Dwarf:        dwarfFile,
		LoadBias:     loadBias,
		symbolTables: symbolTables,
	}

	files.Files = append(files.Files, file)
	return file, nil
}

func (files *Files) SymbolSpans(address VirtualAddress) *elf.Symbol {
	for _, file := range files.Files {
		symbol := file.SymbolSpans(address)
		if symbol != nil {
			return symbol
		}
	}

	return nil
}

func (files *Files) SymbolsByName(name string) []*elf.Symbol {
	results := []*elf.Symbol{}
	for _, file := range files.Files {
		results = append(results, file.SymbolsByName(name)...)
	}

	return results
}

func (files *Files) FunctionEntryContainingAddress(
	address VirtualAddress,
) (
	*dwarf.DebugInfoEntry,
	error,
) {
	for _, file := range files.Files {
		entry, err := file.FunctionEntryContainingAddress(address)
		if entry != nil || err != nil {
			return entry, err
		}
	}

	return nil, nil
}

func (files *Files) FunctionEntriesWithName(
	name string,
) (
	[]*dwarf.DebugInfoEntry,
	error,
) {
	result := []*dwarf.DebugInfoEntry{}
	for _, file := range files.Files {
		entries, err := file.FunctionEntriesWithName(name)
		if err != nil {
			return nil, err
		}

		result = append(result, entries...)
	}

	return result, nil
}

func (files *Files) LineEntryAt(
	address VirtualAddress,
) (
	*dwarf.LineEntry,
	error,
) {
	for _, file := range files.Files {
		entry, err := file.LineEntryAt(address)
		if entry != nil || err != nil {
			return entry, err
		}
	}

	return nil, nil
}

func (files *Files) LineEntriesByLine(
	pathName string,
	line int,
) (
	[]*dwarf.LineEntry,
	error,
) {
	result := []*dwarf.LineEntry{}
	for _, file := range files.Files {
		entries, err := file.LineEntriesByLine(pathName, line)
		if err != nil {
			return nil, err
		}
		result = append(result, entries...)
	}

	return result, nil
}

func (files *Files) ComputeUnwindRulesAt(
	pc VirtualAddress,
) (
	*dwarf.UnwindRules,
	error,
) {
	for _, file := range files.Files {
		rules, err := file.ComputeUnwindRulesAt(pc)
		if rules != nil || err != nil {
			return rules, err
		}
	}
	return nil, nil
}

type File struct {
	*elf.File
	LoadBias uint64

	Dwarf *dwarf.File // optional

	symbolTables []*elf.SymbolTableSection
}

func (file *File) ParseAddress(value string) (VirtualAddress, error) {
	if strings.HasPrefix(value, "elf:") {
		addr, err := strconv.ParseUint(value[4:], 0, 64)
		if err != nil {
			return 0, fmt.Errorf(
				"failed to parse elf file address (%s): %w",
				value,
				err)
		}

		return VirtualAddress(addr + file.LoadBias), nil
	} else {
		addr, err := strconv.ParseUint(value, 0, 64)
		if err != nil {
			return 0, fmt.Errorf(
				"failed to parse virtual address (%s): %w",
				value,
				err)
		}

		return VirtualAddress(addr), nil
	}
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

func (file *File) EntryPointVirtualAddress() VirtualAddress {
	return file.ToVirtualAddress(file.EntryPointFileAddress())
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
