package debugger

import (
	"bytes"
	"encoding/binary"
	"fmt"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/dwarf"
)

type DataKind string

const (
	BoolKind  = DataKind("bool")
	CharKind  = DataKind("char") // signed/unsigned char
	IntKind   = DataKind("int")
	UintKind  = DataKind("uint")
	FloatKind = DataKind("float")

	PointerKind = DataKind("pointer")

	// NOTE: in c++, data member is a single pointer, whereas method member is
	// a pair of pointers (base class pointer, multi-inheritance pointer)
	MemberPointerKind = DataKind("member pointer")

	ArrayKind = DataKind("array") // single dimension array

	StructKind = DataKind("struct") // class/struct
	UnionKind  = DataKind("union")
)

type DataDescriptor struct {
	Pool *DataDescriptorPool

	Kind DataKind

	// NOTE: may not match field's physical data representation
	ByteSize int

	// Only applicable to pointers and arrays
	Value *DataDescriptor

	// Only applicable to arrays
	NumElements int

	// Only applicable to structs/unions
	Fields []*FieldDescriptor

	// NOTE: for multi-dimensional arrays, only the inner-most array descriptor
	// has a non-nil DIE entry.
	DIE *dwarf.DebugInfoEntry

	resolved bool
}

func (descriptor *DataDescriptor) Name() string {
	kind := string(descriptor.Kind)
	if descriptor.Kind == IntKind ||
		descriptor.Kind == UintKind ||
		descriptor.Kind == FloatKind {

		kind += fmt.Sprintf("%d", 8*descriptor.ByteSize)
	}

	return kind
}

func (descriptor *DataDescriptor) IsSimpleValue() bool {
	switch descriptor.Kind {
	case ArrayKind, StructKind, UnionKind:
		return false
	default:
		return true
	}
}

func (descriptor *DataDescriptor) IsCharPointer() bool {
	return descriptor.Kind == PointerKind && descriptor.Value.Kind == CharKind
}

func (descriptor *DataDescriptor) resolveSizeAndValueDescriptor() error {
	if descriptor.resolved {
		return nil
	}
	descriptor.resolved = true

	if descriptor.Kind == PointerKind {
		valueDIE, err := descriptor.DIE.TypeEntry()
		if err != nil {
			return fmt.Errorf("invalid pointer type: %w", err)
		}

		valueDesc, err := descriptor.Pool.Get(valueDIE)
		if err != nil {
			return fmt.Errorf("invalid pointer type: %w", err)
		}

		descriptor.Value = valueDesc
	} else if descriptor.Kind == ArrayKind {
		descs := []*DataDescriptor{descriptor}
		for descs[len(descs)-1].DIE == nil {
			nested := descs[len(descs)-1].Value
			if nested == nil || nested.Kind != ArrayKind {
				panic("should never happen")
			}

			descs = append(descs, nested)
		}

		valueDIE, err := descs[len(descs)-1].DIE.TypeEntry()
		if err != nil {
			return fmt.Errorf("invalid array type: %w", err)
		}

		valueDesc, err := descriptor.Pool.Get(valueDIE)
		if err != nil {
			return fmt.Errorf("invalid array type: %w", err)
		}

		descs[len(descs)-1].Value = valueDesc

		for idx := len(descs) - 1; idx >= 0; idx-- {
			desc := descs[idx]
			desc.ByteSize = desc.NumElements * desc.Value.ByteSize
		}
	} else if descriptor.Kind == StructKind {
		// NOTE: the struct's byte size is provided by its DIE
		for _, field := range descriptor.Fields {
			valueDIE, err := field.DIE.TypeEntry()
			if err != nil {
				return fmt.Errorf("invalid struct field type: %w", err)
			}

			valueDesc, err := descriptor.Pool.Get(valueDIE)
			if err != nil {
				return fmt.Errorf("invalid struct field type: %w", err)
			}

			field.Value = valueDesc

			if field.BitOffset < 0 { // dwarf4's deprecated bit-packed field encoding
				size, ok := field.DIE.Uint(dwarf.DW_AT_byte_size)

				byteSize := int(size)
				if !ok {
					byteSize = valueDesc.ByteSize
				}

				field.BitOffset = 8*byteSize + field.BitOffset

			} else if field.BitSize == 0 { // non-bit-packed field
				field.BitSize = 8 * valueDesc.ByteSize
			}
		}
	}

	return nil
}

type FieldDescriptor struct {
	Pool *DataDescriptorPool

	Name  string
	Value *DataDescriptor

	// Physical data representation
	ByteOffset int // relative to the beginning of the struct
	BitOffset  int // relative to the beginning of the field byte
	BitSize    int

	DIE *dwarf.DebugInfoEntry
}

type DataDescriptorPool struct {
	dieDescriptors map[*dwarf.DebugInfoEntry]*DataDescriptor
}

func NewDataDescriptorPool() *DataDescriptorPool {
	return &DataDescriptorPool{
		dieDescriptors: map[*dwarf.DebugInfoEntry]*DataDescriptor{},
	}
}

func (pool *DataDescriptorPool) Get(
	die *dwarf.DebugInfoEntry,
) (
	*DataDescriptor,
	error,
) {
	descriptor, ok := pool.dieDescriptors[die]
	if ok {
		return descriptor, nil
	}

	descriptor, err := pool.parseDIE(die)
	if err != nil {
		return nil, err
	}

	// Insert parsed descriptor into pool so that self-type pointer references
	// can be resolved.
	pool.dieDescriptors[die] = descriptor

	err = descriptor.resolveSizeAndValueDescriptor()
	if err != nil {
		return nil, err
	}

	return descriptor, nil
}

func (pool *DataDescriptorPool) parseDIE(
	die *dwarf.DebugInfoEntry,
) (
	*DataDescriptor,
	error,
) {
	switch die.Tag {
	case dwarf.DW_TAG_base_type:
		return pool.parseBaseType(die)

	case dwarf.DW_TAG_pointer_type:
		return &DataDescriptor{
			Pool:     pool,
			Kind:     PointerKind,
			ByteSize: 8,
			DIE:      die,
		}, nil

	case dwarf.DW_TAG_ptr_to_member_type:
		return pool.parseMemberPointerType(die)

	case dwarf.DW_TAG_array_type:
		return pool.parseArrayType(die)

	case dwarf.DW_TAG_class_type,
		dwarf.DW_TAG_structure_type,
		dwarf.DW_TAG_union_type:

		return pool.parseStructType(die)

	case dwarf.DW_TAG_enumeration_type,
		dwarf.DW_TAG_typedef,
		dwarf.DW_TAG_const_type,
		dwarf.DW_TAG_volatile_type:

		// NOTE: We'll ignore type qualifiers that don't impact data representation

		base, err := die.TypeEntry()
		if err != nil {
			return nil, fmt.Errorf("invalid type qualifier: %w", err)
		}

		return pool.Get(base)
	}

	return nil, fmt.Errorf("unsupported data type (%s)", die.Tag)
}

func (pool *DataDescriptorPool) parseBaseType(
	die *dwarf.DebugInfoEntry,
) (
	*DataDescriptor,
	error,
) {
	encoding, ok := die.Uint(dwarf.DW_AT_encoding)
	if !ok {
		return nil, fmt.Errorf("base type encoding not found")
	}

	byteSize, ok := die.Uint(dwarf.DW_AT_byte_size)
	if !ok {
		return nil, fmt.Errorf("base type byte size not found")
	}

	var kind DataKind
	switch encoding {
	case dwarf.DW_ATE_boolean:
		kind = BoolKind
		if byteSize != 1 {
			return nil, fmt.Errorf("unsupported bool size (%d)", byteSize)
		}
	case dwarf.DW_ATE_signed_char, dwarf.DW_ATE_unsigned_char:
		kind = CharKind
		if byteSize != 1 {
			return nil, fmt.Errorf("unsupported char size (%d)", byteSize)
		}
	case dwarf.DW_ATE_signed:
		kind = IntKind
		if byteSize != 1 && byteSize != 2 && byteSize != 4 && byteSize != 8 {
			return nil, fmt.Errorf("unsupported int size (%d)", byteSize)
		}
	case dwarf.DW_ATE_unsigned:
		kind = UintKind
		if byteSize != 1 && byteSize != 2 && byteSize != 4 && byteSize != 8 {
			return nil, fmt.Errorf("unsupported uint size (%d)", byteSize)
		}
	case dwarf.DW_ATE_float:
		kind = FloatKind
		if byteSize != 4 && byteSize != 8 {
			return nil, fmt.Errorf("unsupported float size (%d)", byteSize)
		}
	default:
		return nil, fmt.Errorf("unsupported base type encoding (%d)", encoding)
	}

	return &DataDescriptor{
		Pool:     pool,
		Kind:     kind,
		ByteSize: int(byteSize),
		DIE:      die,
	}, nil
}

func (pool *DataDescriptorPool) parseMemberPointerType(
	die *dwarf.DebugInfoEntry,
) (
	*DataDescriptor,
	error,
) {
	base, err := die.TypeEntry()
	if err != nil {
		return nil, fmt.Errorf("invalid member pointer type: %w", err)
	}

	byteSize := 8 // data field

	if base.Tag == dwarf.DW_TAG_subroutine_type { // method field
		byteSize = 16
	}

	return &DataDescriptor{
		Pool:     pool,
		Kind:     MemberPointerKind,
		ByteSize: byteSize,
		DIE:      die,
	}, nil
}

func (pool *DataDescriptorPool) parseArrayType(
	die *dwarf.DebugInfoEntry,
) (
	*DataDescriptor,
	error,
) {
	var outerMost *DataDescriptor
	var prev *DataDescriptor
	for _, child := range die.Children {
		if child.Tag == dwarf.DW_TAG_subrange_type {
			dim, ok := child.Uint(dwarf.DW_AT_upper_bound)
			if !ok {
				return nil, fmt.Errorf("invalid array dimension")
			}

			current := &DataDescriptor{
				Pool:        pool,
				Kind:        ArrayKind,
				NumElements: int(dim + 1),
			}

			if outerMost == nil {
				outerMost = current
			}

			if prev != nil {
				prev.Value = current
			}
			prev = current
		}
	}

	if outerMost == nil {
		return nil, fmt.Errorf("array type has no dimensions")
	}

	prev.DIE = die
	return outerMost, nil
}

func (pool *DataDescriptorPool) parseStructType(
	die *dwarf.DebugInfoEntry,
) (
	*DataDescriptor,
	error,
) {
	byteSize, ok := die.Uint(dwarf.DW_AT_byte_size)
	if !ok {
		return nil, fmt.Errorf("struct type byte size not found")
	}

	fields := []*FieldDescriptor{}
	for _, child := range die.Children {
		if child.Tag != dwarf.DW_TAG_member {
			continue
		}

		name, _, err := child.Name()
		if err != nil {
			return nil, err
		}

		// NOTE: field's data descriptor and non-bit-packed bit size are defer
		// resolved.
		field := &FieldDescriptor{
			Pool: pool,
			Name: name,
			DIE:  child,
		}

		fields = append(fields, field)

		// dwarf4's preferred bit-packed field encoding
		dataBitOffset, ok := child.Uint(dwarf.DW_AT_data_bit_offset)
		if ok {
			bitSize, ok := child.Uint(dwarf.DW_AT_bit_size)
			if !ok {
				return nil, fmt.Errorf(
					"invalid bit-packed field (%s). no bit size",
					name)
			}

			field.ByteOffset = int(dataBitOffset / 8)
			field.BitOffset = int(dataBitOffset % 8)
			field.BitSize = int(bitSize)

			continue
		}

		location, ok := child.Uint(dwarf.DW_AT_data_member_location)
		if !ok {
			return nil, fmt.Errorf("invalid field. no data member location")
		}

		// NOTE: BitSize for non-bit-packed field is uninitialized for now
		field.ByteOffset = int(location)

		// dwarf4's deprecated bit-packed field encoding
		bitOffset, ok := child.Uint(dwarf.DW_AT_bit_offset)
		if ok {
			bitSize, ok := child.Uint(dwarf.DW_AT_bit_size)
			if !ok {
				return nil, fmt.Errorf(
					"invalid bit-packed field (%s). no bit size",
					name)
			}

			// NOTE: The bit offset computed here is relative to the end of the field
			// byte.  The real bit off (relative to the beginning of field byte) is
			// resolved later.
			field.BitOffset = int(-bitOffset - bitSize)
			field.BitSize = int(bitSize)
		}
	}

	kind := StructKind
	if die.Tag == dwarf.DW_TAG_union_type {
		kind = UnionKind
	}

	// NOTE: field value descriptors are defer resolved.
	return &DataDescriptor{
		Pool:     pool,
		Kind:     kind,
		ByteSize: int(byteSize),
		Fields:   fields,
		DIE:      die,
	}, nil
}

type TypedData struct {
	*memory.VirtualMemory

	FormatPrefix string

	*DataDescriptor
	Data []byte

	// NOTE: Only populate by ReadVariable
	dwarf.Location
}

func (data *TypedData) Dereference() (*TypedData, error) {
	return data.dereference(0)
}

func (data *TypedData) dereference(idx int) (*TypedData, error) {
	if data.Kind != PointerKind {
		return nil, fmt.Errorf(
			"%w. cannot deference non-pointer (%s) type",
			ErrInvalidInput,
			data.Kind)
	}

	addr, err := data.DecodeSimpleValue()
	if err != nil {
		return nil, err
	}

	address := addr.(VirtualAddress)
	address += VirtualAddress(idx * data.Value.ByteSize)

	value := make([]byte, data.Value.ByteSize)

	n, err := data.Read(address, value)
	if err != nil {
		return nil, fmt.Errorf("failed to read referenced value: %w", err)
	}
	if n != data.Value.ByteSize {
		return nil, fmt.Errorf("failed to read referenced value. not enough bytes")
	}

	return &TypedData{
		VirtualMemory:  data.VirtualMemory,
		FormatPrefix:   "*",
		DataDescriptor: data.Value,
		Data:           value,
	}, nil
}

func (data *TypedData) Index(idx int) (*TypedData, error) {
	if data.Kind == PointerKind {
		return data.dereference(idx)
	}

	if data.Kind != ArrayKind {
		return nil, fmt.Errorf(
			"%w. cannot index into %s type",
			ErrInvalidInput,
			data.Kind)
	}

	if idx < 0 || data.NumElements <= idx {
		return nil, fmt.Errorf("%w. index out of bound", ErrInvalidInput)
	}

	start := idx * data.Value.ByteSize
	end := start + data.Value.ByteSize

	return &TypedData{
		VirtualMemory:  data.VirtualMemory,
		FormatPrefix:   fmt.Sprintf("[%d]", idx),
		DataDescriptor: data.Value,
		Data:           data.Data[start:end],
	}, nil
}

func (data *TypedData) FieldByName(name string) (*TypedData, error) {
	if data.Kind != StructKind && data.Kind != UnionKind {
		return nil, fmt.Errorf(
			"%w. cannot access field for non-struct/union (%s) type",
			ErrInvalidInput,
			data.Kind)
	}

	var match *FieldDescriptor
	for _, field := range data.Fields {
		if field.Name == name {
			match = field
			break
		}
	}

	if match == nil {
		return nil, fmt.Errorf("field (%s) not found", name)
	}

	return data.fieldData(match)
}

func (data *TypedData) fieldData(match *FieldDescriptor) (*TypedData, error) {
	appender := &BitsAppender{}
	appender.AppendSlice(
		data.Data[match.ByteOffset:],
		match.BitOffset,
		match.BitSize)
	fieldData := appender.Finalize()

	// Pad bit-packed fields to the expected size
	for len(fieldData) < match.Value.ByteSize {
		fieldData = append(fieldData, 0)
	}

	name := match.Name
	if name == "" {
		name = "<unnamed>"
	}

	return &TypedData{
		VirtualMemory:  data.VirtualMemory,
		FormatPrefix:   "." + name,
		DataDescriptor: match.Value,
		Data:           fieldData,
	}, nil
}

func decodeSimpleValue[T any](data []byte, value T) (interface{}, int, error) {
	n, err := binary.Decode(data, binary.LittleEndian, &value)
	return value, n, err
}

// This returns a correctly sized golang value for base types, or a
// VirtualAddress for pointer / member pointer.  This returns error for
// array / struct / union.
func (data *TypedData) DecodeSimpleValue() (interface{}, error) {
	var value interface{}
	var n int
	var err error
	switch data.Kind {
	case ArrayKind, StructKind, UnionKind:
		return nil, fmt.Errorf("cannot decode %s into simple value", data.Kind)
	case PointerKind, MemberPointerKind:
		// NOTE: We'll ignore the second pointer in method members
		value, n, err = decodeSimpleValue(data.Data, VirtualAddress(0))
	case BoolKind:
		value, n, err = decodeSimpleValue(data.Data, false)
	case CharKind:
		value, n, err = decodeSimpleValue(data.Data, byte(0))
	case IntKind:
		switch data.ByteSize {
		case 1:
			value, n, err = decodeSimpleValue(data.Data, int8(0))
		case 2:
			value, n, err = decodeSimpleValue(data.Data, int16(0))
		case 4:
			value, n, err = decodeSimpleValue(data.Data, int32(0))
		case 8:
			value, n, err = decodeSimpleValue(data.Data, int64(0))
		default:
			panic("should never happen")
		}
	case UintKind:
		switch data.ByteSize {
		case 1:
			value, n, err = decodeSimpleValue(data.Data, uint8(0))
		case 2:
			value, n, err = decodeSimpleValue(data.Data, uint16(0))
		case 4:
			value, n, err = decodeSimpleValue(data.Data, uint32(0))
		case 8:
			value, n, err = decodeSimpleValue(data.Data, uint64(0))
		default:
			panic("should never happen")
		}
	case FloatKind:
		switch data.ByteSize {
		case 4:
			value, n, err = decodeSimpleValue(data.Data, float32(0))
		case 8:
			value, n, err = decodeSimpleValue(data.Data, float64(0))
		default:
			panic("should never happen")
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to decode simple value: %w", err)
	}

	if data.Kind == MemberPointerKind {
		if n != 8 {
			return nil, fmt.Errorf(
				"failed to decode simple value. incorrect size (%d != 8)",
				n)
		}
	} else if n != data.ByteSize {
		return nil, fmt.Errorf(
			"failed to decode simple value. incorrect size (%d != %d)",
			n,
			data.ByteSize)
	}

	return value, nil
}

func (data *TypedData) ReadCString() (string, error) {
	if !data.IsCharPointer() {
		return "", fmt.Errorf("cannot read c string. not char pointer")
	}

	addr, err := data.DecodeSimpleValue()
	if err != nil {
		return "", err
	}

	address := addr.(VirtualAddress)

	result := []byte{}
	buffer := make([]byte, 1024)

	for {
		n, err := data.Read(address, buffer)
		if err != nil {
			return "", fmt.Errorf("failed to read c string: %w", err)
		}
		if n == 0 {
			return "", fmt.Errorf("failed to read c string. read zero bytes")
		}

		chunk := buffer[:n]

		idx := bytes.Index(chunk, []byte{0})
		if idx == -1 {
			result = append(result, chunk...)
			address += VirtualAddress(len(chunk))
			continue
		}

		if idx > 0 {
			result = append(result, chunk[:idx]...) // don't include \0
		}

		return string(result), nil
	}
}

func (data *TypedData) Format(indent string) string {
	switch data.Kind {
	case StructKind, UnionKind:
		result := fmt.Sprintf("%s%s: {\n", indent, data.FormatPrefix)

		nextIndent := indent + "  "
		for _, field := range data.Fields {
			element, err := data.fieldData(field)
			if err != nil {
				panic(err) // should never happen
			}

			result += element.Format(nextIndent) + ",\n"
		}

		result += fmt.Sprintf("%s}\n", indent)
		return result

	case ArrayKind:
		result := fmt.Sprintf("%s%s: [\n", indent, data.FormatPrefix)

		nextIndent := indent + "  "
		for i := 0; i < data.NumElements; i++ {
			element, err := data.Index(i)
			if err != nil {
				panic(err)
			}

			result += element.Format(nextIndent) + ",\n"
		}

		result += fmt.Sprintf("%s]\n", indent)
		return result

	default:
		value, err := data.DecodeSimpleValue()
		if err != nil {
			panic(err) // should never happen
		}

		detail := ""
		if data.Kind == CharKind {
			detail = fmt.Sprintf(" (%s)", string([]byte{value.(byte)}))
		} else if data.IsCharPointer() {
			str, err := data.ReadCString()
			if err == nil {
				detail = " (" + str + ")"
			}
		}

		return fmt.Sprintf(
			"%s%s (%s): %v%s",
			indent,
			data.FormatPrefix,
			data.Name(),
			value,
			detail)
	}
}
