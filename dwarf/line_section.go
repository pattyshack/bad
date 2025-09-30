package dwarf

import (
	"encoding/binary"
	"fmt"
	"path"
	"strings"

	"github.com/pattyshack/bad/elf"
)

const (
	DW_LNS_copy               = 0x01
	DW_LNS_advance_pc         = 0x02
	DW_LNS_advance_line       = 0x03
	DW_LNS_set_file           = 0x04
	DW_LNS_set_column         = 0x05
	DW_LNS_negate_stmt        = 0x06
	DW_LNS_set_basic_block    = 0x07
	DW_LNS_const_add_pc       = 0x08
	DW_LNS_fixed_advance_pc   = 0x09
	DW_LNS_set_prologue_end   = 0x0a
	DW_LNS_set_epilogue_begin = 0x0b
	DW_LNS_set_isa            = 0x0c

	DW_LNE_end_sequence      = 0x01
	DW_LNE_set_address       = 0x02
	DW_LNE_define_file       = 0x03
	DW_LNE_set_discriminator = 0x04
	DW_LNE_lo_user           = 0x80
	DW_LNE_hi_user           = 0xff
)

type LineSection struct {
	LineTables map[SectionOffset]*LineTable
}

func NewLineSection(file *elf.File) (*LineSection, error) {
	section := file.GetSection(ElfDebugLineSection)
	if section == nil {
		return &LineSection{}, nil
	}

	content, err := section.RawContent()
	if err != nil {
		return nil, fmt.Errorf("failed to read elf .debug_line section: %w", err)
	}

	tables := map[SectionOffset]*LineTable{}

	decode := NewCursor(file.ByteOrder(), content)
	for !decode.HasReachedEnd() {
		table, err := parseLineTable(decode)
		if err != nil {
			return nil, err
		}

		tables[table.SectionOffset] = table
	}

	return &LineSection{
		LineTables: tables,
	}, nil
}

type FileEntry struct {
	*LineTable

	Name             string
	DirIndex         uint64 // 0-based (0 holds the compilation directory)
	ModificationTime uint64
	Length           uint64
}

func (entry FileEntry) String() string {
	return entry.Path()
}

func (entry FileEntry) Path() string {
	return path.Join(entry.IncludedDirectories[entry.DirIndex], entry.Name)
}

type LineTable struct {
	byteOrder binary.ByteOrder
	*CompileUnit

	SectionOffset

	DefaultIsStatement bool
	LineBase           int8
	LineRange          uint8
	OpCodeBase         uint8

	IncludedDirectories []string
	FileEntries         []*FileEntry

	Content []byte
}

func parseLineTable(
	decode *Cursor,
) (
	*LineTable,
	error,
) {
	start := decode.Position

	length, err := decode.U32()
	if err != nil {
		return nil, fmt.Errorf("failed to decode line table length: %w", err)
	}

	end := decode.Position + int(length)

	version, err := decode.U16()
	if err != nil {
		return nil, fmt.Errorf("failed to decode line table version: %w", err)
	}
	if version != 4 {
		return nil, fmt.Errorf(
			"failed to parse line table. dwarf version %d not supported",
			version)
	}

	headerLength, err := decode.U32()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to decode line table header length: %w",
			err)
	}
	expectedContentStart := decode.Position + int(headerLength)

	minInstructionLen, err := decode.U8()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to decode line table minimum instruction length: %w",
			err)
	}
	// Must be 1 on x64 (e.g., int3)
	if minInstructionLen != 1 {
		return nil, fmt.Errorf(
			"unsupported line table minimum instruction length (%d)",
			minInstructionLen)
	}

	maxOperationsPerInstruction, err := decode.U8()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to decode line table maximum operations per instruction: %w",
			err)
	}
	// Must be 1 on x64 (non-VLIW architecture)
	if maxOperationsPerInstruction != 1 {
		return nil, fmt.Errorf(
			"unsupported line table maximum operations per instruction (%d)",
			maxOperationsPerInstruction)
	}

	defaultIsStatement, err := decode.U8()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to decode line table default is statement: %w",
			err)
	}

	lineBase, err := decode.S8()
	if err != nil {
		return nil, fmt.Errorf("failed to decode line table line base: %w", err)
	}

	lineRange, err := decode.U8()
	if err != nil {
		return nil, fmt.Errorf("failed to decode line table line range: %w", err)
	}

	opCodeBase, err := decode.U8()
	if err != nil {
		return nil, fmt.Errorf("failed to decode line table op code base: %w", err)
	}
	if opCodeBase > 13 {
		return nil, fmt.Errorf("invalid line table op code base (%d)", opCodeBase)
	}

	stdNumOperands := []uint8{0, 1, 1, 1, 1, 0, 0, 0, 1, 0, 0, 1}
	for idx, expected := range stdNumOperands[:opCodeBase-1] {
		num, err := decode.U8()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to decode line table standard op code (%d) num operand: %w",
				idx+1,
				err)
		}
		if num != expected {
			return nil, fmt.Errorf(
				"invalid num operand (%d != %d) for standard op code (%d)",
				num,
				expected,
				idx+1)
		}
	}

	included := []string{""} // NOTE: reserve space for compilation dir
	for {
		dir, err := decode.String()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to decode line table included directories: %w",
				err)
		}

		if dir == "" {
			break
		}

		included = append(included, dir)
	}

	table := &LineTable{
		byteOrder:           decode.ByteOrder,
		SectionOffset:       SectionOffset(start),
		DefaultIsStatement:  defaultIsStatement != 0,
		LineBase:            lineBase,
		LineRange:           lineRange,
		OpCodeBase:          opCodeBase,
		IncludedDirectories: included,
	}

	for {
		shouldContinue, err := table.parseAndAddFileEntry(decode, true)
		if err != nil {
			return nil, err
		}

		if !shouldContinue {
			break
		}
	}

	if decode.Position != expectedContentStart {
		return nil, fmt.Errorf(
			"failed to decode line table header. unexpected length")
	}

	content, err := decode.Bytes(end - decode.Position)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read line table content bytes: %w",
			err)
	}
	table.Content = content

	return table, nil
}

func (table *LineTable) parseAndAddFileEntry(
	decode *Cursor,
	expectsTerminalMarker bool,
) (
	bool, // true if valid entry was parsed
	error,
) {
	name, err := decode.String()
	if err != nil {
		return false, fmt.Errorf(
			"failed to decode line table file entry name: %w",
			err)
	}

	if name == "" {
		if expectsTerminalMarker {
			return false, nil
		}

		return false, fmt.Errorf(
			"failed to decode line table file entry name. empty string")
	}

	dirIndex, err := decode.ULEB128(64)
	if err != nil {
		return false, fmt.Errorf(
			"failed to decode line table file entry directory index: %w",
			err)
	}

	if dirIndex >= uint64(len(table.IncludedDirectories)) {
		return false, fmt.Errorf(
			"invalid line table file entry directory index. out of bound")
	}

	modTime, err := decode.ULEB128(64)
	if err != nil {
		return false, fmt.Errorf(
			"failed to decode line table file entry modification time: %w",
			err)
	}

	length, err := decode.ULEB128(64)
	if err != nil {
		return false, fmt.Errorf(
			"failed to decode line table file entry length: %w",
			err)
	}

	table.FileEntries = append(
		table.FileEntries,
		&FileEntry{
			LineTable:        table,
			Name:             name,
			DirIndex:         dirIndex,
			ModificationTime: modTime,
			Length:           length,
		})
	return true, nil
}

func (table *LineTable) setCompileUnit(
	unit *CompileUnit,
	compilationDir string,
) error {
	if table.CompileUnit != nil {
		return fmt.Errorf("line table's compile unit already set")
	}
	table.CompileUnit = unit

	for idx, dir := range table.IncludedDirectories {
		if idx == 0 {
			table.IncludedDirectories[0] = compilationDir
		} else if !strings.HasPrefix(dir, "/") {
			table.IncludedDirectories[idx] = compilationDir + "/" + dir
		}
	}

	return nil
}

func (table *LineTable) Iterator() (*LineEntry, error) {
	return newLineIterator(table, NewCursor(table.byteOrder, table.Content))
}

type LineEntry struct {
	elf.FileAddress
	FileIndex       uint64 // 1-based instead of 0-based
	Line            int64
	Column          uint64
	IsStatement     bool
	BasicBlockStart bool
	EndSequence     bool
	PrologueEnd     bool
	EpilogueBegin   bool
	ISA             uint64 // X64 does not care about this register
	Discriminator   uint64

	*FileEntry

	reinitialize     bool
	shouldResetFlags bool

	table      *LineTable
	operations *Cursor
}

func (entry *LineEntry) CompileUnit() *CompileUnit {
	return entry.table.CompileUnit
}

func (entry *LineEntry) String() string {
	return fmt.Sprintf("%s:%d:%d", entry.Path(), entry.Line, entry.Column)
}

func newLineIterator(table *LineTable, cursor *Cursor) (*LineEntry, error) {
	entry := &LineEntry{
		table:        table,
		operations:   cursor,
		reinitialize: true,
	}
	return entry.advance()
}

func (entry *LineEntry) clone() *LineEntry {
	cloned := *entry
	cloned.operations = entry.operations.Clone()
	return &cloned
}

func (entry *LineEntry) initialize() {
	entry.FileAddress = 0
	entry.FileIndex = 1
	entry.Line = 1
	entry.Column = 0
	entry.IsStatement = entry.table.DefaultIsStatement
	entry.BasicBlockStart = false
	entry.EndSequence = false
	entry.PrologueEnd = false
	entry.EpilogueBegin = false
	entry.ISA = 0
	entry.Discriminator = 0

	entry.reinitialize = false
	entry.shouldResetFlags = false
}

func (entry *LineEntry) resetFlags() {
	entry.BasicBlockStart = false
	entry.PrologueEnd = false
	entry.EpilogueBegin = false
	entry.Discriminator = 0

	entry.reinitialize = false
	entry.shouldResetFlags = false
}

func (entry *LineEntry) Next() (*LineEntry, error) {
	nextEntry := entry.clone()
	return nextEntry.advance()
}

// NOTE: error is only returned for unexpected error.  (nil, nil) indicates end.
func (entry *LineEntry) advance() (*LineEntry, error) {
	if entry.reinitialize {
		entry.initialize()
	} else if entry.shouldResetFlags {
		entry.resetFlags()
	}

	for !entry.operations.HasReachedEnd() {
		shouldEmitted, err := entry.execute()
		if err != nil {
			return nil, err
		}

		if shouldEmitted {
			idx := entry.FileIndex - 1
			if idx >= uint64(len(entry.table.FileEntries)) {
				return nil, fmt.Errorf("out of bound line entry file index")
			}

			entry.FileEntry = entry.table.FileEntries[idx]
			return entry, nil
		}
	}

	return nil, nil
}

func (entry *LineEntry) execute() (bool, error) {
	opCode, err := entry.operations.U8()
	if err != nil {
		return false, fmt.Errorf("failed to decode op code: %w", err)
	}

	if opCode >= entry.table.OpCodeBase {
		entry.executeSpecialOp(opCode - entry.table.OpCodeBase)
		return true, nil
	}

	switch opCode {
	case 0:
		return entry.executeExtendedOp()

	case DW_LNS_copy:
		entry.shouldResetFlags = true
		return true, nil

	case DW_LNS_advance_pc:
		addressDelta, err := entry.operations.ULEB128(64)
		if err != nil {
			return false, fmt.Errorf(
				"failed to decode DW_LNS_advance_pc operand: %w",
				err)
		}

		entry.FileAddress += elf.FileAddress(addressDelta)

	case DW_LNS_advance_line:
		lineDelta, err := entry.operations.SLEB128(64)
		if err != nil {
			return false, fmt.Errorf(
				"failed to decode DW_LNS_advance_line operand: %w",
				err)
		}

		entry.Line += lineDelta

	case DW_LNS_set_file:
		index, err := entry.operations.ULEB128(64)
		if err != nil {
			return false, fmt.Errorf(
				"failed to decode DW_LNS_set_file operand: %w",
				err)
		}

		entry.FileIndex = index

	case DW_LNS_set_column:
		column, err := entry.operations.ULEB128(64)
		if err != nil {
			return false, fmt.Errorf(
				"failed to decode DW_LNS_set_column operand: %w",
				err)
		}

		entry.Column = column

	case DW_LNS_negate_stmt:
		entry.IsStatement = !entry.IsStatement

	case DW_LNS_set_basic_block:
		entry.BasicBlockStart = true

	case DW_LNS_const_add_pc:
		addressDelta := (255 - entry.table.OpCodeBase) / entry.table.LineRange
		entry.FileAddress += elf.FileAddress(addressDelta)

	case DW_LNS_fixed_advance_pc:
		addressDelta, err := entry.operations.U16()
		if err != nil {
			return false, fmt.Errorf(
				"failed to decode DW_LNS_fixed_advance_pc operand: %w",
				err)
		}

		entry.FileAddress += elf.FileAddress(addressDelta)

	case DW_LNS_set_prologue_end:
		entry.PrologueEnd = true

	case DW_LNS_set_epilogue_begin:
		entry.EpilogueBegin = true

	case DW_LNS_set_isa:
		isa, err := entry.operations.ULEB128(64)
		if err != nil {
			return false, fmt.Errorf(
				"failed to decode DW_LNS_set_isa operand: %w",
				err)
		}

		entry.ISA = isa

	default:
		return false, fmt.Errorf("unknown line op code (%d)", opCode)
	}

	return false, nil
}

func (entry *LineEntry) executeExtendedOp() (bool, error) {
	expectedLength, err := entry.operations.ULEB128(64)
	if err != nil {
		return false, fmt.Errorf("failed to decode extended op length: %w", err)
	}

	start := entry.operations.Position

	opCode, err := entry.operations.U8()
	if err != nil {
		return false, fmt.Errorf("failed to decode extended op code: %w", err)
	}

	switch opCode {
	case DW_LNE_end_sequence:
		entry.EndSequence = true
		entry.reinitialize = true
		return true, nil

	case DW_LNE_set_address:
		address, err := entry.operations.U64()
		if err != nil {
			return false, fmt.Errorf(
				"failed to decode DW_LNE_set_address operand: %w",
				err)
		}

		entry.FileAddress = elf.FileAddress(address)

	case DW_LNE_define_file:
		_, err := entry.table.parseAndAddFileEntry(entry.operations, false)
		if err != nil {
			return false, fmt.Errorf(
				"DW_LNE_define_file operation failed: %w",
				err)
		}

	case DW_LNE_set_discriminator:
		discriminator, err := entry.operations.ULEB128(64)
		if err != nil {
			return false, fmt.Errorf(
				"failed to decode DW_LNE_set_discriminator: %w",
				err)
		}

		entry.Discriminator = discriminator

	default:
		return false, fmt.Errorf("unknown line extended op code (%d)", opCode)
	}

	length := entry.operations.Position - start
	if length != int(expectedLength) {
		return false, fmt.Errorf(
			"invalid line extended op code encoding. unexpected length (%d != %d)",
			length,
			expectedLength)
	}

	return false, nil
}

func (entry *LineEntry) executeSpecialOp(index uint8) {
	addressDelta := index / entry.table.LineRange
	entry.FileAddress += elf.FileAddress(addressDelta)

	lineDelta := int64(entry.table.LineBase) + int64(index%entry.table.LineRange)
	entry.Line += lineDelta

	entry.shouldResetFlags = true
}
