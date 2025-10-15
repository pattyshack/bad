package expression

import (
	"bytes"
	"encoding/binary"
	"fmt"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/dwarf"
)

// NOTE: The debugger implements this interface
type EvaluationContext interface {
	Memory() *memory.VirtualMemory

	DescriptorPool() *DataDescriptorPool

	ReadInspectFrameVariableOrFunction(name string) (*TypedData, error)

	InvokeMallocInCurrentThread(size int) (VirtualAddress, error)

	InvokeInCurrentThread(*TypedData, []*TypedData) (*TypedData, error)

	GetEvaluatedResult(idx int) (*EvaluatedResult, error)
}

type TypedData struct {
	*memory.VirtualMemory

	FormatPrefix string

	*DataDescriptor

	// Data physical location. Not applicable to function kinds.  For methods,
	// the data refers to the receiver.
	Address   VirtualAddress
	BitOffset int
	BitSize   int

	ImplicitValue interface{}

	// Only applicable to function kinds. The index matches the signatures index.
	FunctionAddresses []VirtualAddress

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

	return &TypedData{
		VirtualMemory:  data.VirtualMemory,
		FormatPrefix:   "*",
		DataDescriptor: data.Value,
		Address:        address,
		BitOffset:      0,
		BitSize:        8 * data.Value.ByteSize,
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

	address := data.Address + VirtualAddress(start)

	return &TypedData{
		VirtualMemory:  data.VirtualMemory,
		FormatPrefix:   fmt.Sprintf("[%d]", idx),
		DataDescriptor: data.Value,
		Address:        address,
		BitOffset:      0,
		BitSize:        8 * data.ByteSize,
	}, nil
}

func (data *TypedData) FieldOrMethodByName(name string) (*TypedData, error) {
	if data.Kind != StructKind && data.Kind != UnionKind {
		return nil, fmt.Errorf(
			"%w. cannot access field/method for non-struct/union (%s) type",
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

	if match != nil {
		return data.fieldData(match)
	}

	descriptor, addresses, err := data.DataDescriptor.Pool.GetMethod(
		data.DIE,
		name)
	if err != nil {
		return nil, err
	}

	if descriptor == nil {
		return nil, fmt.Errorf("field/method (%s) not found", name)
	}

	if data.BitOffset != 0 ||
		data.BitSize%8 != 0 ||
		data.BitSize/8 != data.ByteSize {
		panic("should never happen")
	}

	return &TypedData{
		VirtualMemory:  data.VirtualMemory,
		FormatPrefix:   "." + name,
		DataDescriptor: descriptor,

		Address:   data.Address,
		BitOffset: data.BitOffset,
		BitSize:   data.BitSize,

		FunctionAddresses: addresses,
	}, nil
}

func (data *TypedData) fieldData(match *FieldDescriptor) (*TypedData, error) {
	name := match.Name
	if name == "" {
		name = "<unnamed>"
	}

	address := data.Address + VirtualAddress(match.ByteOffset)
	return &TypedData{
		VirtualMemory:  data.VirtualMemory,
		FormatPrefix:   "." + name,
		DataDescriptor: match.Value,
		Address:        address,
		BitOffset:      match.BitOffset,
		BitSize:        match.BitSize,
	}, nil
}

func (data *TypedData) Bytes() ([]byte, error) {
	if data.ImplicitValue != nil {
		bytes := make([]byte, data.ByteSize)
		n, err := binary.Encode(bytes, binary.LittleEndian, data.ImplicitValue)
		if err != nil {
			return nil, fmt.Errorf("failed to encode value: %w", err)
		}
		if n != data.ByteSize {
			return nil, fmt.Errorf(
				"failed to encode value. incorrect number of bytes")
		}

		return bytes, nil
	}

	dataSize := (data.BitOffset + data.BitSize + 7) / 8

	storageData := make([]byte, dataSize)
	n, err := data.Read(data.Address, storageData)
	if err != nil {
		return nil, fmt.Errorf("failed to encode value: %w", err)
	}
	if n != dataSize {
		return nil, fmt.Errorf("failed to encode value. incorrect number of bytes")
	}

	appender := &BitsAppender{}
	appender.AppendSlice(
		storageData,
		data.BitOffset,
		data.BitSize)
	materializedData := appender.Finalize()

	// Pad bit-packed fields to the expected size
	for len(materializedData) < data.ByteSize {
		materializedData = append(materializedData, 0)
	}

	return materializedData, nil
}

func decodeSimpleValue[T any](data []byte, value T) (interface{}, int, error) {
	n, err := binary.Decode(data, binary.LittleEndian, &value)
	return value, n, err
}

// This returns a correctly sized golang value for base types, or a
// VirtualAddress for pointer / member pointer.  This returns error for
// array / struct / union.
func (data *TypedData) DecodeSimpleValue() (interface{}, error) {
	if data.ImplicitValue != nil {
		return data.ImplicitValue, nil
	}

	switch data.Kind {
	case ArrayKind, StructKind, UnionKind, FunctionKind, MethodKind, VoidKind:
		return nil, fmt.Errorf("cannot decode %s into simple value", data.Kind)
	}

	materializedData, err := data.Bytes()
	if err != nil {
		return nil, err
	}

	var value interface{}
	var n int
	switch data.Kind {
	case ArrayKind, StructKind, UnionKind, FunctionKind, MethodKind:
		return nil, fmt.Errorf("cannot decode %s into simple value", data.Kind)
	case PointerKind, MemberPointerKind:
		// NOTE: We'll ignore the second pointer in method members
		value, n, err = decodeSimpleValue(materializedData, VirtualAddress(0))
	case BoolKind:
		value, n, err = decodeSimpleValue(materializedData, false)
	case CharKind:
		value, n, err = decodeSimpleValue(materializedData, byte(0))
	case IntKind:
		switch data.ByteSize {
		case 1:
			value, n, err = decodeSimpleValue(materializedData, int8(0))
		case 2:
			value, n, err = decodeSimpleValue(materializedData, int16(0))
		case 4:
			value, n, err = decodeSimpleValue(materializedData, int32(0))
		case 8:
			value, n, err = decodeSimpleValue(materializedData, int64(0))
		default:
			panic("should never happen")
		}
	case UintKind:
		switch data.ByteSize {
		case 1:
			value, n, err = decodeSimpleValue(materializedData, uint8(0))
		case 2:
			value, n, err = decodeSimpleValue(materializedData, uint16(0))
		case 4:
			value, n, err = decodeSimpleValue(materializedData, uint32(0))
		case 8:
			value, n, err = decodeSimpleValue(materializedData, uint64(0))
		default:
			panic("should never happen")
		}
	case FloatKind:
		switch data.ByteSize {
		case 4:
			value, n, err = decodeSimpleValue(materializedData, float32(0))
		case 8:
			value, n, err = decodeSimpleValue(materializedData, float64(0))
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

func (data *TypedData) MethodReceiverPointer(
	signature *SignatureDescriptor,
) *TypedData {
	return &TypedData{
		VirtualMemory:  data.VirtualMemory,
		FormatPrefix:   "[receiver]",
		DataDescriptor: signature.Parameters[0].DataDescriptor,
		ImplicitValue:  data.Address,
	}
}

func (data *TypedData) SelectMatchingSignature(
	arguments []*TypedData,
) (
	*SignatureDescriptor,
	VirtualAddress,
	error,
) {
	if data.Kind != FunctionKind && data.Kind != MethodKind {
		return nil, 0, fmt.Errorf(
			"selecting signature on non-callable type (%s)",
			data.Kind)
	}

	match := []int{}
	for idx, signature := range data.Signatures {
		if signature.Matches(arguments) {
			match = append(match, idx)
		}
	}

	if len(match) == 0 {
		return nil, 0, fmt.Errorf("no matching %s", data.Kind)
	}

	if len(match) > 1 {
		return nil, 0, fmt.Errorf("ambiguous %s call", data.Kind)
	}

	idx := match[0]
	return data.Signatures[idx], data.FunctionAddresses[idx], nil
}

func (data *TypedData) Format(indent string) string {
	switch data.Kind {
	case VoidKind:
		return indent + "(void)"
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

		result += fmt.Sprintf("%s}", indent)
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

		result += fmt.Sprintf("%s]", indent)
		return result

	case FunctionKind, MethodKind:
		result := fmt.Sprintf("%s%s (%s):",
			indent,
			data.FormatPrefix,
			data.Kind)

		if data.Kind == MethodKind {
			result += fmt.Sprintf("\n%s  receiver (*%s): %s",
				indent,
				data.Value.TypeName(),
				data.Address)
		}

		for funcIdx, addr := range data.FunctionAddresses {
			signature := data.Signatures[funcIdx]

			funcType := "func("
			count := 0
			for paramIdx, param := range signature.Parameters {
				if signature.IsMethod && paramIdx == 0 {
					continue
				}

				if count > 1 {
					funcType += ", "
				}
				funcType += param.TypeName()
				count += 1
			}

			funcType += ")"

			if signature.Return != nil {
				funcType += " " + signature.Return.TypeName()
			}

			result += fmt.Sprintf("\n%s  %s: %s", indent, addr, funcType)
		}

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
			data.TypeName(),
			value,
			detail)
	}
}

func Evaluate(ctx EvaluationContext, expression string) (*TypedData, error) {
	return Parse(newLexer(expression), newReducer(ctx))
}
