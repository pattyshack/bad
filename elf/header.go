// Based on linux's man page, elf.h, golang's debug/elf package,
// and the elf 1.2 spec.
package elf

import (
	"fmt"
)

var (
	// EI_MAG0 - EI_MAG3
	IdentifierMagic = []byte{
		0x7f, // ELFMAG0
		'E',  // ELFMAG1
		'L',  // ELFMAG2
		'F',  // ELFMAG3
	}
)

const (
	MaxNumProgramHeaderEntries = 0xffff // PN_XNUM
	MaxNumSectionHeaderEntries = 0xff00 // SHN_LORESERVE

	SectionStringTableIndexNotDefined = 0 // SHN_UNDEF

	IdentifierVersion = 1 // EI_CURRENT
	ABIVersion        = 0
	FormatVersion     = 1 // EV_CURRENT

	ElfIdentifierSize           = 16
	Elf64HeaderSize             = 64
	Elf64SectionHeaderEntrySize = 64
	Elf64ProgramHeaderEntrySize = 56
	Elf64SymbolEntrySize        = 24

	// NOTE: Although Elf64_Nhdr is defined, it looks like elf64 files in general
	// still encode notes using Elf32_Nhdr.
	NoteHeaderSize = 12
)

// EI_CLASS
type Class byte

const (
	ClassNone = Class(0) // ELFCLASSNONE
	Class32   = Class(1) // ELFCLASS32
	Class64   = Class(2) // ELFCLASS64
)

func (class Class) String() string {
	switch class {
	case ClassNone:
		return "ClassNone"
	case Class32:
		return "Class32"
	case Class64:
		return "Class64"
	default:
		return fmt.Sprintf("ClassUnknown(%d)", class)
	}
}

// EI_DATA
type DataEncoding byte

const (
	DataEncodingNone                       = DataEncoding(0) // ELFDATANONE
	DataEncodingTwosComplementLittleEndian = DataEncoding(1) // ELFDATA2LSB
	DataEncodingTwosComplementBigEndian    = DataEncoding(2) // ELFDATA2MSB
)

func (encoding DataEncoding) String() string {
	switch encoding {
	case DataEncodingNone:
		return "DataEncodingNone"
	case DataEncodingTwosComplementLittleEndian:
		return "TwosComplementLittleEndian"
	case DataEncodingTwosComplementBigEndian:
		return "TwosComplementBigEndian"
	default:
		return fmt.Sprintf("DataEncodingUnknown(%d)", encoding)
	}
}

// EI_OSABI
// NOTE: golang's debug/elf.OSABI defines a more complete list
type OperatingSystemABI byte

const (
	OperatingSystemABIUnixSystemV = OperatingSystemABI(0) // ELFOSABI_NONE
	OperatingSystemABILinux       = OperatingSystemABI(3) // ELFOSABI_LINUX
)

func (osAbi OperatingSystemABI) String() string {
	switch osAbi {
	case OperatingSystemABIUnixSystemV:
		return "UnixSystemV"
	case OperatingSystemABILinux:
		return "Linux"
	default:
		return fmt.Sprintf("OperatingSystemABIUnknown(%d)", osAbi)
	}
}

// e_type
type FileType uint16

const (
	FileTypeNone         = FileType(0) // ET_NONE
	FileTypeRelocatable  = FileType(1) // ET_REL
	FileTypeExecutable   = FileType(2) // ET_EXEC
	FileTypeSharedObject = FileType(3) // ET_DYN
	FileTypeCore         = FileType(4) // ET_CORE
)

func (ft FileType) String() string {
	switch ft {
	case FileTypeNone:
		return "FileTypeNone"
	case FileTypeRelocatable:
		return "Relocatable"
	case FileTypeExecutable:
		return "Executable"
	case FileTypeSharedObject:
		return "SharedObject"
	case FileTypeCore:
		return "Core"
	default:
		return fmt.Sprintf("FileTypeUnknown(%d)", ft)
	}
}

type ProgramType uint32

// see debug/elf for a more complete list
const (
	ProgramNull            = ProgramType(0)          // PT_NULL
	ProgramLoadable        = ProgramType(1)          // PT_LOAD
	ProgramDynamicLinking  = ProgramType(2)          // PT_DYNAMIC
	ProgramInterpreterPath = ProgramType(3)          // PT_INTERP
	ProgramNote            = ProgramType(4)          // PT_NOTE
	ProgramHeaderInfo      = ProgramType(6)          // PT_PHDR
	ProgramGNUStack        = ProgramType(0x6474e551) // PT_GNU_STACK
)

func (segType ProgramType) String() string {
	switch segType {
	case ProgramNull:
		return "ProgramNull"
	case ProgramLoadable:
		return "Loadable"
	case ProgramDynamicLinking:
		return "DynamicLinking"
	case ProgramInterpreterPath:
		return "InterpreterPath"
	case ProgramNote:
		return "Note"
	case ProgramHeaderInfo:
		return "HeaderInfo"
	case ProgramGNUStack:
		return "GNUStack"
	default:
		return fmt.Sprintf("ProgramUnknown(%d)", segType)
	}
}

type ProgramFlags uint32

const (
	ProgramFlagExecutableBit = ProgramFlags(0x1)
	ProgramFlagWritableBit   = ProgramFlags(0x2)
	ProgramFlagReadableBit   = ProgramFlags(0x4)
)

func (bits ProgramFlags) String() string {
	if bits > 7 {
		return fmt.Sprintf("%#x", uint32(bits))
	}

	rwx := []byte{'-', '-', '-'}
	if bits&ProgramFlagReadableBit != 0 {
		rwx[0] = 'r'
	}

	if bits&ProgramFlagWritableBit != 0 {
		rwx[1] = 'w'
	}

	if bits&ProgramFlagExecutableBit != 0 {
		rwx[2] = 'x'
	}

	return string(rwx)
}

type SectionType uint32

const (
	SectionTypeNull                  = SectionType(0)  // SHT_NULL
	SectionTypeProgramDefinedInfo    = SectionType(1)  // SHT_PROGBITS
	SectionTypeSymbolTable           = SectionType(2)  // SHT_SYMTAB
	SectionTypeStringTable           = SectionType(3)  // SHT_STRTAB
	SectionTypeRelocationWithAddends = SectionType(4)  // SHT_RELA
	SectionTypeSymbolHashTable       = SectionType(5)  // SHT_HASH
	SectionTypeDynamic               = SectionType(6)  // SHT_DYNAMIC
	SectionTypeNote                  = SectionType(7)  // SHT_NOTE
	SectionTypeNoSpace               = SectionType(8)  // SHT_NOBITS
	SectionTypeRelocationNoAddends   = SectionType(9)  // SHT_REL
	SectionTypeDynamicSymbolTable    = SectionType(11) // SHT_DYNSYM
)

func (stype SectionType) String() string {
	switch stype {
	case SectionTypeNull:
		return "SectionTypeNull"
	case SectionTypeProgramDefinedInfo:
		return "ProgramDefinedInfo"
	case SectionTypeSymbolTable:
		return "SymbolTable"
	case SectionTypeStringTable:
		return "StringTable"
	case SectionTypeRelocationWithAddends:
		return "RelocationWithAddends"
	case SectionTypeSymbolHashTable:
		return "SymbolHashTable"
	case SectionTypeDynamic:
		return "Dynamic"
	case SectionTypeNote:
		return "Note"
	case SectionTypeNoSpace:
		return "NoSpace"
	case SectionTypeRelocationNoAddends:
		return "RelocationNoAddends"
	case SectionTypeDynamicSymbolTable:
		return "DynamicSymbolTable"
	default:
		return fmt.Sprintf("SectionTypeUnknown(%d)", stype)
	}
}

type SectionFlags uint64

const (
	SectionContainsWritableData         = SectionFlags(0x1)   // SHF_WRITE
	SectionOccupiesMemory               = SectionFlags(0x2)   // SHF_ALLOC
	SectionContainsInstructions         = SectionFlags(0x4)   // SHF_EXECINSTR
	SectionMayBeMerged                  = SectionFlags(0x10)  // SHF_MERGE
	SectionContainsStrings              = SectionFlags(0x20)  // SHF_STRINGS
	SectionInfoHoldsSectionIndex        = SectionFlags(0x40)  // SHF_INFO_LINK
	SectionRequiresSpecialOrdering      = SectionFlags(0x80)  // SHF_LINK_ORDER
	SectionRequiresOsSpecificProcessing = SectionFlags(0x100) // SHF_OS_NONCONFORMING
	SectionIsGroupMember                = SectionFlags(0x200) // SHF_GROUP
	SectionContainsTLSData              = SectionFlags(0x400) // SHF_TLS
	SectionIsCompressed                 = SectionFlags(0x800) // SHF_COMPRESSED
)

func (flags SectionFlags) String() string {
	result := make([]byte, 11)
	for i := 0; i < 11; i++ {
		result[i] = '-'
	}

	if flags&SectionContainsWritableData != 0 {
		result[0] = 'w'
	}
	if flags&SectionOccupiesMemory != 0 {
		result[1] = 'a'
	}
	if flags&SectionContainsInstructions != 0 {
		result[2] = 'x'
	}
	if flags&SectionMayBeMerged != 0 {
		result[3] = 'm'
	}
	if flags&SectionContainsStrings != 0 {
		result[4] = 's'
	}
	if flags&SectionInfoHoldsSectionIndex != 0 {
		result[5] = 'i'
	}
	if flags&SectionRequiresSpecialOrdering != 0 {
		result[6] = 'l'
	}
	if flags&SectionRequiresOsSpecificProcessing != 0 {
		result[7] = 'o'
	}
	if flags&SectionIsGroupMember != 0 {
		result[8] = 'g'
	}
	if flags&SectionContainsTLSData != 0 {
		result[9] = 't'
	}
	if flags&SectionIsCompressed != 0 {
		result[10] = 'c'
	}

	return string(result)
}

// e_machine
// NOTE: golang's debug/elf.Machine defines a more complete list of machine
// types.
type MachineArchitecture uint16

const (
	MachineArchitectureNone   = MachineArchitecture(0)  // EM_NONE
	MachineArchitectureX86_64 = MachineArchitecture(62) // EM_X86_64
)

func (arch MachineArchitecture) String() string {
	switch arch {
	case MachineArchitectureNone:
		return "MachineArchitectureNone"
	case MachineArchitectureX86_64:
		return "x86-64"
	default:
		return fmt.Sprintf("MachineArchitectureUnknown(%d)", arch)
	}
}

// The bottom 4 bits of st_info
type SymbolType byte

func SymbolInfoToType(info byte) SymbolType {
	return SymbolType(info & 0xf)
}

const (
	SymbolTypeNone                     = SymbolType(0) // STT_NOTYPE
	SymbolTypeObject                   = SymbolType(1) // STT_OBJECT
	SymbolTypeFunction                 = SymbolType(2) // STT_FUNC
	SymbolTypeSection                  = SymbolType(3) // STT_SECTION
	SymbolTypeSourceFile               = SymbolType(4) // STT_FILE
	SymbolTypeUninitializedCommonBlock = SymbolType(5) // STT_COMMON
	SymbolTypeTLSObject                = SymbolType(6) // STT_TLS
)

func (st SymbolType) String() string {
	switch st {
	case SymbolTypeNone:
		return "NoType"
	case SymbolTypeObject:
		return "Object"
	case SymbolTypeFunction:
		return "Function"
	case SymbolTypeSection:
		return "Section"
	case SymbolTypeSourceFile:
		return "SourceFile"
	case SymbolTypeUninitializedCommonBlock:
		return "UinitializedCommonBlock"
	case SymbolTypeTLSObject:
		return "TLSObject"
	default:
		return fmt.Sprintf("SymbolTypeUnknown(%d)", st)
	}
}

// The top 4 bits of st_info
type SymbolBinding byte

func SymbolInfoToBinding(info byte) SymbolBinding {
	return SymbolBinding(info >> 4)
}

const (
	SymbolBindingLocal  = SymbolBinding(0) // STB_LOCAL
	SymbolBindingGlobal = SymbolBinding(1) // STB_GLOBAL
	SymbolBindingWeak   = SymbolBinding(2) // STB_WEAK
)

func (sb SymbolBinding) String() string {
	switch sb {
	case SymbolBindingLocal:
		return "Local"
	case SymbolBindingGlobal:
		return "Global"
	case SymbolBindingWeak:
		return "Weak"
	default:
		return fmt.Sprintf("SymbolBindingUnknown(%d)", sb)
	}
}

type SymbolVisibility byte

const (
	SymbolVisibilityDefault   = SymbolVisibility(0) // STV_DEFAULT
	SymbolVisibilityInternal  = SymbolVisibility(1) // STV_INTERNAL
	SymbolVisibilityHidden    = SymbolVisibility(2) // STV_HIDDEN
	SymbolVisibilityProtected = SymbolVisibility(3) // STV_PROTECTED
)

func (vis SymbolVisibility) String() string {
	switch vis {
	case SymbolVisibilityDefault:
		return "Default"
	case SymbolVisibilityInternal:
		return "Internal"
	case SymbolVisibilityHidden:
		return "Hidden"
	case SymbolVisibilityProtected:
		return "Protected"
	default:
		return fmt.Sprintf("SymbolVisibilityUnknown(%d)", vis)
	}
}

type SectionIndex uint16

const (
	SectionIndexUndefined = SectionIndex(0)
	SectionIndexAbsolute  = SectionIndex(0xfff1)

	SectionStringTableName = ".shstrtab"
	StringTableName        = ".strtab"
)

// Header structs matching c's elf64 header definitions.  These are only used
// for (de-)serialization.

// e_ident
type Identifier struct {
	Magic              [4]byte // EI_MAG0 ... EI_MAG3
	Class                      // EI_CLASS
	DataEncoding               // EI_DATA
	IdentifierVersion  byte    // EI_VERSION
	OperatingSystemABI         // EI_OSABI
	ABIVersion         byte    // EI_ABIVERSION
	Padding            [7]byte // EI_PAD
}

// Elf64_Ehdr
type ElfHeader struct {
	Identifier                           // e_ident[EI_NIDENT]
	FileType                             // e_type
	MachineArchitecture                  // e_machine
	FormatVersion           uint32       // e_version
	EntryPointAddress       uint64       // e_entry
	ProgramHeaderOffset     uint64       // e_phoff
	SectionHeaderOffset     uint64       // e_shoff
	ArchitectureFlags       uint32       // e_flags
	ElfHeaderSize           uint16       // e_ehsize
	ProgramHeaderEntrySize  uint16       // e_phentsize
	NumProgramHeaderEntries uint16       // e_phnum
	SectionHeaderEntrySize  uint16       // e_shentsize
	NumSectionHeaderEntries uint16       // e_shnum
	SectionStringTableIndex SectionIndex // e_shstrndx
}

// Elf64_Phdr
type ProgramHeaderEntry struct {
	ProgramType            // p_type
	ProgramFlags           // p_flags
	ContentOffset   uint64 // p_offset
	VirtualAddress  uint64 // p_vaddr
	PhysicalAddress uint64 // p_paddr
	FileImageSize   uint64 // filesz
	MemoryImageSize uint64 // p_memsz
	Alignment       uint64 // p_align
}

// Elf64_Shdr
type SectionHeaderEntry struct {
	NameIndex        uint32 // sh_name
	SectionType             // sh_type
	SectionFlags            // sh_flags
	Address          uint64 // sh_addr
	Offset           uint64 // sh_offset
	Size             uint64 // sh_size
	Link             uint32 // sh_link
	Info             uint32 // sh_info
	AddressAlignment uint64 // sh_addralign
	EntrySize        uint64 // sh_entsize
}

// Elf64_Sym
type SymbolEntry struct {
	NameIndex        uint32 // st_name
	Info             byte   // st_info.  (4 bits st_bind, 4 bits st_type)
	SymbolVisibility        // st_other
	SectionIndex            // st_shndx
	Value            uint64 // st_value
	Size             uint64 // st_size
}

// NOTE: Although Elf64_Nhdr is defined, it looks like notes in elf64 files
// are still encoded using Elf32_Nhdr.
// Elf32_Nhdr
type NoteHeader struct {
	NameSize        uint32
	DescriptionSize uint32
	Type            uint32
}
