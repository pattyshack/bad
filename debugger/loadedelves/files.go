package loadedelves

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"strings"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/dwarf"
	"github.com/pattyshack/bad/elf"
)

const (
	elfDynamicSection = ".dynamic"

	debugRendezvousSize = 40
	linkMapEntrySize    = 32
	maxLinuxPathLength  = 4096
)

// r_debug in link.h
type debugRendezvous struct {
	// r_version - 2 for dlmopen, 1 for dlopen / in general
	Version int32

	_ int32 // word alignment padding

	// r_map - link list of link_map entries
	LinkMap VirtualAddress

	// r_brk - address of a no-op notify function. Set break site at this address
	// to watch for changes to r_debug / link map entries.  The function is
	// called before and after each change.
	NotifyFunction VirtualAddress

	// r_state:
	// - RT_CONSISTENT (0) - dynamic linker is in consistent state
	// - RT_ADD (1) - dynamic linker is adding a library
	// - RT_DELETE (2) - dynamic linker is deleting a library
	State int32

	_ int32 // word alignment padding

	// r_ldbase - dynamic linker location
	LinkerLocation VirtualAddress
}

// NOTE: The actual link_map c struct in link.h is much larger, but we only
// care about the first 4 fields.
type linkMapEntry struct {
	Location   VirtualAddress // l_addr - elf entry loaded address
	NameString VirtualAddress // l_name - zero-terminated string
	LdLocation VirtualAddress // l_ld - entry's .dynamic section loaded address
	NextEntry  VirtualAddress // l_next - next link map entry or nil
}

type Files struct {
	memory *memory.VirtualMemory

	Executable *File
	loaded     map[string]*File
}

func NewFiles(mem *memory.VirtualMemory) *Files {
	return &Files{
		memory: mem,
		loaded: map[string]*File{},
	}
}

func (files *Files) Files() []*File {
	result := make([]*File, 0, len(files.loaded))
	for _, file := range files.loaded {
		result = append(result, file)
	}

	sort.Slice(
		result,
		func(i int, j int) bool {
			return result[i].LoadBias < result[j].LoadBias
		})

	return result
}

func (files *Files) LoadExecutable(pid int) (*File, error) {
	if files.Executable != nil {
		return files.Executable, nil
	}

	file, err := newExecutableFile(pid)
	if err != nil {
		return nil, err
	}

	files.Executable = file
	files.loaded[""] = file
	return file, nil
}

func (files *Files) UpdateFiles() (VirtualAddress, bool, error) {
	notifyAddress, loadedLibs, err := files.ReadRendezvousInfo()
	if err != nil {
		return 0, false, err
	}

	modified := false
	for name, _ := range files.loaded {
		if name == "" {
			continue
		}

		_, ok := loadedLibs[name]
		if !ok {
			delete(files.loaded, name)
			modified = true
		}
	}

	for name, address := range loadedLibs {
		_, ok := files.loaded[name]
		if ok {
			continue
		}

		var file *File
		var err error
		if name == VDSOFileName {
			file, err = newVDSOFile(files.memory, address)
		} else {
			file, err = newDynamicallyLoadedFile(name, address)
		}

		if err != nil {
			return 0, false, fmt.Errorf("failed to load elf file (%s): %w", name, err)
		}

		modified = true
		files.loaded[name] = file
	}

	return notifyAddress, modified, nil
}

// NOTE: the dynamic linker's rendezvous information is only valid after the
// program has reached the main entry point.
func (files *Files) ReadRendezvousInfo() (
	VirtualAddress, // notify function address
	map[string]VirtualAddress, // loaded libraries
	error,
) {
	addr, libs, err := files._readRendezvousInfo()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read rendezvous info: %w", err)
	}
	return addr, libs, nil
}

func (files *Files) _readRendezvousInfo() (
	VirtualAddress, // notify function address
	map[string]VirtualAddress, // loaded libraries
	error,
) {
	rendezvousAddress, err := files.LocateRendezvousAddress()
	if err != nil {
		return 0, nil, err
	}

	rendezvousBytes := make([]byte, debugRendezvousSize)
	n, err := files.memory.Read(rendezvousAddress, rendezvousBytes)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read debug rendezvous: %w", err)
	}
	if n != debugRendezvousSize {
		panic("should never happen")
	}

	rendezvous := &debugRendezvous{}
	n, err = binary.Decode(rendezvousBytes, binary.LittleEndian, rendezvous)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to decode debug rendezvous: %w", err)
	}
	if n != debugRendezvousSize {
		panic("should never happen")
	}

	if rendezvous.Version < 1 || 2 < rendezvous.Version {
		return 0, nil, fmt.Errorf(
			"invalid debug rendezvous version (%d)",
			rendezvous.Version)
	}

	if rendezvous.State < 0 || 2 < rendezvous.State {
		return 0, nil, fmt.Errorf(
			"invalid debug rendezvous state (%d)",
			rendezvous.State)
	}

	loadedLibs := map[string]VirtualAddress{}

	linkMapBytes := make([]byte, linkMapEntrySize)
	linkMap := &linkMapEntry{}

	nameBytes := make([]byte, maxLinuxPathLength)

	address := rendezvous.LinkMap
	for address != 0 {
		n, err := files.memory.Read(address, linkMapBytes)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read link map entry: %w", err)
		}
		if n != linkMapEntrySize {
			panic("should never happen")
		}

		n, err = binary.Decode(linkMapBytes, binary.LittleEndian, linkMap)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to decode link map entry: %w", err)
		}
		if n != linkMapEntrySize {
			panic("should never happen")
		}

		// NOTE: number of bytes read could be less than the full buffer size
		n, err = files.memory.Read(linkMap.NameString, nameBytes)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read link map entry name: %w", err)
		}

		end := bytes.IndexByte(nameBytes[:n], 0)
		if end == -1 {
			return 0, nil, fmt.Errorf("link map entry name not zero terminated")
		}

		loadedLibs[string(nameBytes[:end])] = linkMap.Location

		address = linkMap.NextEntry
	}

	return rendezvous.NotifyFunction, loadedLibs, nil
}

func (files *Files) LocateRendezvousAddress() (VirtualAddress, error) {
	section := files.Executable.GetSection(elfDynamicSection)
	if section == nil {
		return 0, fmt.Errorf("elf .dynamic section not found")
	}

	header := section.Header()
	sectionAddress := files.Executable.ToVirtualAddress(
		elf.FileAddress(header.Address))

	sectionBytes := make([]byte, int(header.Size))

	n, err := files.memory.Read(sectionAddress, sectionBytes)
	if err != nil {
		return 0, fmt.Errorf("cannot read loaded .dynamic section: %w", err)
	}
	if n != len(sectionBytes) {
		panic("should never happen")
	}

	numEntries := int(header.Size) / elf.Elf64DynamicEntrySize
	dynamicEntries := make([]elf.DynamicEntry, numEntries)

	n, err = binary.Decode(sectionBytes, binary.LittleEndian, dynamicEntries)
	if err != nil {
		return 0, fmt.Errorf("cannot decode loaded .dynamic section: %w", err)
	}
	if n != len(sectionBytes) {
		panic("should never happen")
	}

	for _, entry := range dynamicEntries {
		if entry.DynamicTag == elf.DynamicTagDebug && entry.ValueOrAddress != 0 {
			address := VirtualAddress(entry.ValueOrAddress)

			return address, nil
		}
	}

	return 0, ErrRendezvousAddressNotFound
}

func (files *Files) EntryPoint() VirtualAddress {
	return files.Executable.ToVirtualAddress(
		files.Executable.EntryPointFileAddress())
}

func (files *Files) ParseAddress(value string) (VirtualAddress, error) {
	chunks := strings.Split(value, ":")
	if len(chunks) == 1 {
		addr, err := strconv.ParseUint(value, 0, 64)
		if err != nil {
			return 0, fmt.Errorf(
				"failed to parse virtual address (%s): %w",
				value,
				err)
		}
		return VirtualAddress(addr), nil
	}

	if chunks[0] != "elf" || len(chunks) > 3 {
		return 0, fmt.Errorf("failed to parse virtual address (%s)", value)
	}

	fileName := ""
	fileAddrStr := chunks[1]
	if len(chunks) == 3 {
		fileName = chunks[1]
		fileAddrStr = chunks[2]
	}

	fileAddr, err := strconv.ParseUint(fileAddrStr, 0, 64)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to parse file address (%s): %w",
			value,
			err)
	}

	for _, file := range files.loaded {
		if file.FileName == fileName {
			return file.ToVirtualAddress(elf.FileAddress(fileAddr)), nil
		}
	}

	return 0, fmt.Errorf("elf file not found (%s)", fileName)
}

func (files *Files) ToVirtualAddress(
	elfFile *elf.File,
	fileAddress elf.FileAddress,
) (
	VirtualAddress,
	error,
) {
	for _, file := range files.loaded {
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

func (files *Files) SymbolSpans(address VirtualAddress) *elf.Symbol {
	for _, file := range files.loaded {
		symbol := file.SymbolSpans(address)
		if symbol != nil {
			return symbol
		}
	}

	return nil
}

func (files *Files) SymbolsByName(name string) []*elf.Symbol {
	results := []*elf.Symbol{}
	for _, file := range files.loaded {
		results = append(results, file.SymbolsByName(name)...)
	}

	return results
}

func (files *Files) FunctionEntryContainingAddress(
	address VirtualAddress,
) (
	*File,
	*dwarf.DebugInfoEntry,
	error,
) {
	for _, file := range files.loaded {
		entry, err := file.FunctionEntryContainingAddress(address)
		if entry != nil || err != nil {
			return file, entry, err
		}
	}

	return nil, nil, nil
}

func (files *Files) FunctionEntriesWithName(
	name string,
) (
	[]*dwarf.DebugInfoEntry,
	error,
) {
	result := []*dwarf.DebugInfoEntry{}
	for _, file := range files.loaded {
		entries, err := file.FunctionEntriesWithName(name)
		if err != nil {
			return nil, err
		}

		result = append(result, entries...)
	}

	return result, nil
}

func (files *Files) LocalVariableEntries(
	pc VirtualAddress,
) (
	map[string]*dwarf.DebugInfoEntry,
	error,
) {
	for _, file := range files.loaded {
		entry, err := file.LocalVariableEntries(pc)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			return entry, nil
		}
	}

	return nil, nil
}

func (files *Files) VariableEntryWithName(
	pc VirtualAddress,
	name string,
) (
	*dwarf.DebugInfoEntry,
	error,
) {
	for _, file := range files.loaded {
		entry, err := file.VariableEntryWithName(pc, name)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			return entry, nil
		}
	}
	return nil, nil
}

func (files *Files) LineEntryAt(
	address VirtualAddress,
) (
	*dwarf.LineEntry,
	error,
) {
	for _, file := range files.loaded {
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
	for _, file := range files.loaded {
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
	for _, file := range files.loaded {
		rules, err := file.ComputeUnwindRulesAt(pc)
		if rules != nil || err != nil {
			return rules, err
		}
	}
	return nil, nil
}
