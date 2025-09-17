package bad

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pattyshack/bad/elf"
	"github.com/pattyshack/bad/procfs"
)

const (
	symbolTableName        = ".symtab"
	dynamicSymbolTableName = ".dynsym"
)

type LoadedElfFile struct {
	*elf.File
	LoadBias uint64

	symbolTables []*elf.SymbolTableSection
}

func loadElf(pid int) (*LoadedElfFile, error) {
	content, err := os.ReadFile(procfs.GetExecutableSymlinkPath(pid))
	if err != nil {
		return nil, fmt.Errorf("failed to read elf file: %w", err)
	}

	file, err := elf.ParseBytes(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse elf file: %w", err)
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

	loadBias := loadedEntryPointAddress - file.EntryPointAddress

	symbolTables := []*elf.SymbolTableSection{}

	section, ok := file.GetSection(symbolTableName)
	if ok {
		symbolTables = append(symbolTables, section.(*elf.SymbolTableSection))
	}

	section, ok = file.GetSection(dynamicSymbolTableName)
	if ok {
		symbolTables = append(symbolTables, section.(*elf.SymbolTableSection))
	}

	return &LoadedElfFile{
		File:         file,
		LoadBias:     loadBias,
		symbolTables: symbolTables,
	}, nil
}

func (file *LoadedElfFile) ParseAddress(value string) (VirtualAddress, error) {
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

func (file *LoadedElfFile) ToFileAddress(
	address VirtualAddress,
) elf.FileAddress {
	return elf.FileAddress(uint64(address) - file.LoadBias)
}

func (file *LoadedElfFile) ToVirtualAddress(
	address elf.FileAddress,
) VirtualAddress {
	return VirtualAddress(uint64(address) + file.LoadBias)
}

func (file *LoadedElfFile) SymbolsByName(name string) []*elf.Symbol {
	results := []*elf.Symbol{}
	for _, table := range file.symbolTables {
		results = append(results, table.SymbolsByName(name)...)
	}

	return results
}

func (file *LoadedElfFile) SymbolAt(address VirtualAddress) *elf.Symbol {
	fileAddr := file.ToFileAddress(address)

	for _, table := range file.symbolTables {
		symbol := table.SymbolAt(fileAddr)
		if symbol != nil {
			return symbol
		}
	}

	return nil
}

func (file *LoadedElfFile) SymbolSpans(address VirtualAddress) *elf.Symbol {
	fileAddr := file.ToFileAddress(address)

	for _, table := range file.symbolTables {
		symbol := table.SymbolSpans(fileAddr)
		if symbol != nil {
			return symbol
		}
	}

	return nil
}
