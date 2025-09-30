package dwarf

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pattyshack/bad/elf"
)

const (
	signExtensionMask = ^uint64(0)
)

type Cursor struct {
	binary.ByteOrder

	Content  []byte
	Position int
}

func NewCursor(
	byteOrder binary.ByteOrder,
	content []byte,
) *Cursor {
	return &Cursor{
		ByteOrder: byteOrder,
		Content:   content,
		Position:  0,
	}
}

func (cursor *Cursor) Clone() *Cursor {
	return &Cursor{
		ByteOrder: cursor.ByteOrder,
		Content:   cursor.Content,
		Position:  cursor.Position,
	}
}

func (cursor *Cursor) remaining() []byte {
	return cursor.Content[cursor.Position:]
}

func (cursor *Cursor) HasReachedEnd() bool {
	return len(cursor.remaining()) == 0
}

func (cursor *Cursor) Seek(offset int, whence int) (int, error) {
	pos := 0
	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = cursor.Position + offset
	case io.SeekEnd:
		pos = len(cursor.Content) + offset
	}

	if pos < 0 || len(cursor.Content) < pos {
		return 0, fmt.Errorf("out of bound seek (%d)", pos)
	}

	cursor.Position = pos
	return pos, nil
}

func (cursor *Cursor) Bytes(size int) ([]byte, error) {
	content := cursor.remaining()
	if size < 0 || len(content) < size {
		return nil, fmt.Errorf(
			"out of bound slice %d [%d:%d+%d]",
			len(content),
			cursor.Position,
			cursor.Position,
			size)
	}

	content = content[:size]
	cursor.Position += size
	return content, nil
}

func (cursor *Cursor) String() (string, error) {
	content := cursor.remaining()
	if len(content) == 0 {
		return "", fmt.Errorf("cannot decode string: %w", io.EOF)
	}

	end := -1
	for idx, char := range content {
		if char == 0 {
			end = idx
			break
		}
	}

	if end == -1 {
		return "", fmt.Errorf("string not terminated (%d)", cursor.Position)
	}

	cursor.Position += end + 1 // +1 for trailing \0

	// exclude trailing \0
	return string(content[:end]), nil
}

func (cursor *Cursor) decode(out interface{}, name string) error {
	n, err := binary.Decode(cursor.remaining(), cursor.ByteOrder, out)
	if err != nil {
		return fmt.Errorf(
			"failed to decode %s (%d): %w",
			name,
			cursor.Position,
			err)
	}

	cursor.Position += n
	return nil
}

func (cursor *Cursor) U8() (uint8, error) {
	var result uint8
	err := cursor.decode(&result, "U8")
	return result, err
}

func (cursor *Cursor) S8() (int8, error) {
	var result int8
	err := cursor.decode(&result, "S8")
	return result, err
}

func (cursor *Cursor) U16() (uint16, error) {
	var result uint16
	err := cursor.decode(&result, "U16")
	return result, err
}

func (cursor *Cursor) S16() (int16, error) {
	var result int16
	err := cursor.decode(&result, "S16")
	return result, err
}

func (cursor *Cursor) U32() (uint32, error) {
	var result uint32
	err := cursor.decode(&result, "U32")
	return result, err
}

func (cursor *Cursor) S32() (int32, error) {
	var result int32
	err := cursor.decode(&result, "S32")
	return result, err
}

func (cursor *Cursor) U64() (uint64, error) {
	var result uint64
	err := cursor.decode(&result, "U64")
	return result, err
}

func (cursor *Cursor) S64() (int64, error) {
	var result int64
	err := cursor.decode(&result, "S64")
	return result, err
}

func (cursor *Cursor) uleb128(
	bitSize int,
) (
	uint64, // decoded uint
	int, // shift
	byte, // upper byte
	error,
) {
	content := cursor.remaining()
	if len(content) == 0 {
		return 0, 0, 0, fmt.Errorf("cannot decode LEB128: %w", io.EOF)
	}

	result := uint64(0)
	shift := 0
	numBytes := 0
	current := byte(0)
	for len(content) > 0 && bitSize > shift {
		current = content[0]
		content = content[1:]

		result |= uint64(current&0x7f) << shift
		shift += 7
		numBytes += 1

		if (current & 0x80) == 0 {
			cursor.Position += numBytes
			return result, shift, current, nil
		}
	}

	return 0, 0, 0, fmt.Errorf("LEB128 not terminated (%d)", cursor.Position)
}

func (cursor *Cursor) ULEB128(bitSize int) (uint64, error) {
	result, _, _, err := cursor.uleb128(bitSize)
	if err != nil {
		return 0, err
	}

	return result, err
}

func (cursor *Cursor) SLEB128(bitSize int) (int64, error) {
	result, shift, upper, err := cursor.uleb128(bitSize)
	if err != nil {
		return 0, err
	}

	if shift < bitSize && (upper&0x40) != 0 {
		result |= signExtensionMask << shift
	}

	return int64(result), nil
}

func (cursor *Cursor) Value(
	currentUnit *CompileUnit,
	format Format,
) (
	interface{},
	error,
) {
	val, err := cursor.value(currentUnit, format)
	if err != nil {
		return nil, fmt.Errorf("failed to decode value (%s): %w", format, err)
	}

	return val, nil
}

func (cursor *Cursor) value(
	currentUnit *CompileUnit,
	format Format,
) (
	interface{},
	error,
) {
	uintField, err := cursor.uintField(format)
	if err != nil {
		return nil, err
	}

	switch format {
	case DW_FORM_addr:
		return elf.FileAddress(uintField), nil

	case DW_FORM_sec_offset:
		return SectionOffset(uintField), nil

	case DW_FORM_flag:
		return uintField != 0, nil

	case DW_FORM_flag_present: // NOTE: this has no encoded value bytes
		return true, nil

	case DW_FORM_data1,
		DW_FORM_data2,
		DW_FORM_data4,
		DW_FORM_data8,
		DW_FORM_udata:

		return uintField, nil

	case DW_FORM_sdata:
		return cursor.SLEB128(64)

	case DW_FORM_block1,
		DW_FORM_block2,
		DW_FORM_block4,
		DW_FORM_block,
		DW_FORM_exprloc:

		return cursor.Bytes(int(uintField))

	case DW_FORM_string:
		return cursor.String()

	case DW_FORM_strp:
		return currentUnit.StringAt(SectionOffset(uintField))

	case DW_FORM_ref1,
		DW_FORM_ref2,
		DW_FORM_ref4,
		DW_FORM_ref8,
		DW_FORM_ref_udata:

		addr := currentUnit.Start + SectionOffset(uintField)

		return newDebugInfoEntryReference(currentUnit.File, addr), nil

	case DW_FORM_ref_addr:
		return newDebugInfoEntryReference(
			currentUnit.File,
			SectionOffset(uintField)), nil

	case DW_FORM_indirect:
		return cursor.Value(currentUnit, Format(uintField))

	default:
		// DW_FORM_ref_sig8 and other unknown format types
		return nil, fmt.Errorf("unsupported format (%s)", format)
	}
}

// This return 0 if the format's first field does not involve uint.
func (cursor *Cursor) uintField(format Format) (uint64, error) {
	switch format {

	case DW_FORM_flag,
		DW_FORM_data1,
		DW_FORM_block1,
		DW_FORM_ref1:

		val, err := cursor.U8()
		return uint64(val), err

	case DW_FORM_data2,
		DW_FORM_block2,
		DW_FORM_ref2:

		val, err := cursor.U16()
		return uint64(val), err

	case DW_FORM_sec_offset,
		DW_FORM_data4,
		DW_FORM_block4,
		DW_FORM_strp,
		DW_FORM_ref4:

		val, err := cursor.U32()
		return uint64(val), err

	case DW_FORM_addr,
		DW_FORM_data8,
		DW_FORM_ref8:

		return cursor.U64()

	case DW_FORM_block,
		DW_FORM_exprloc,
		DW_FORM_ref_udata:

		return cursor.ULEB128(32)

	case DW_FORM_udata,
		DW_FORM_indirect:

		return cursor.ULEB128(64)
	}

	return 0, nil
}
