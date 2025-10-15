package expression

import (
	"fmt"
	"strings"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/loadedelves"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/dwarf"
	"github.com/pattyshack/bad/elf"
)

type DataKind string

// NOTE: For simplicity, we'll disallow x87 parameter classes
type ParameterClass string

const (
	VoidKind  = DataKind("void")
	BoolKind  = DataKind("bool")
	CharKind  = DataKind("char") // signed/unsigned char
	IntKind   = DataKind("int")
	UintKind  = DataKind("uint")
	FloatKind = DataKind("float")

	// NOTE: For simplicity, we'll treat references as non-nil pointers.
	PointerKind = DataKind("pointer")

	// NOTE: in c++, data member is a single pointer, whereas method member is
	// a pair of pointers (base class pointer, multi-inheritance pointer)
	MemberPointerKind = DataKind("member pointer")

	ArrayKind = DataKind("array") // single dimension array

	StructKind = DataKind("struct") // class/struct
	UnionKind  = DataKind("union")

	FunctionKind = DataKind("function")
	MethodKind   = DataKind("method")

	NoClass      = ParameterClass("no class")
	IntegerClass = ParameterClass("integer class")
	SSEClass     = ParameterClass("sse class")
	MemoryClass  = ParameterClass("memory class")
)

type DataDescriptor struct {
	Pool *DataDescriptorPool

	Kind DataKind

	// NOTE: may not match field's physical data representation
	ByteSize int

	// Only applicable to pointers, arrays, and method. For method,
	// the value refers to the receiver.
	Value *DataDescriptor

	// Only applicable to arrays
	NumElements int

	// Only applicable to functions, methods, structs and unions
	Name string

	// Only applicable to structs and unions
	Fields []*FieldDescriptor

	// Only applicable to functions/methods
	Signatures []*SignatureDescriptor

	// NOTE: For multi-dimensional arrays, only the inner-most array descriptor
	// has a non-nil DIE entry. For function kind, the DIE (if available) is in
	// the signature descriptor.
	DIE *dwarf.DebugInfoEntry

	resolved bool
}

// See x86-64 SYS V ABI, section 3.2.3 Parameter Passing for additional details
func (descriptor *DataDescriptor) ParameterClasses() ([]ParameterClass, error) {
	switch descriptor.Kind {
	case VoidKind:
		return nil, nil
	case BoolKind, CharKind, IntKind, UintKind, PointerKind:
		if descriptor.ByteSize > 8 {
			return nil, fmt.Errorf(
				"unsupported integer size (%d)",
				descriptor.ByteSize)
		}
		return []ParameterClass{IntegerClass}, nil
	case FloatKind:
		if descriptor.ByteSize > 8 {
			return nil, fmt.Errorf(
				"unsupported float size (%d)",
				descriptor.ByteSize)
		}
		return []ParameterClass{SSEClass}, nil

	case ArrayKind, StructKind, UnionKind:
		isNonTrivial, err := descriptor.IsNonTrivialForCalls()
		if err != nil {
			return nil, err
		} else if isNonTrivial {
			// This should never happen in valid function signature since SYS V ABI
			// passes NTFPOC argument by invisible reference (the object is replaced
			// by a pointer)
			return nil, fmt.Errorf("NTFPOC types are not supported")
		}

		// NOTE: SYS V ABI states that we should classify up to 8 eightbytes, then
		// perform the merge step. However, since we don't support __m128, __m256,
		// x87, __int128, and complex type, we can greatly simplify this (struct
		// larger than 2 eightbyte is automatically converted to MemoryClass due
		// to merge rule 5c and lack of X87/X87UP/COMPLEX_X87/SSEUP support).

		if descriptor.ByteSize > 16 || descriptor.HasUnalignedFields() {
			return []ParameterClass{MemoryClass}, nil
		}

		classes := []ParameterClass{NoClass}
		if descriptor.ByteSize > 8 {
			classes = append(classes, NoClass)
		}

		if descriptor.Kind == ArrayKind {
			valueClasses, err := descriptor.Value.ParameterClasses()
			if err != nil {
				return nil, fmt.Errorf("unsupported array value: %w", err)
			}

			if len(valueClasses) == 1 {
				for idx, _ := range classes {
					// NOTE: This implicitly applies rule 4a. for value smaller than
					// eightbyte (packing multiple values of the same type into eightbyte
					// will result in the same class)
					classes[idx] = valueClasses[0]
				}
			} else if len(valueClasses) == 2 {
				classes = valueClasses
			} else {
				panic("should never happen")
			}
		} else { // class / struct / union
			for _, field := range descriptor.Fields {
				classifyField(classes, field, 0)
			}
		}

		for _, class := range classes {
			if class == NoClass {
				panic("should never happen")
			}
		}

		// Merge rule 5a.  Note that rules 5b, 5c and 5d are not applicable since
		// we don't support X87/X87UP/COMPLEX_X87/SSEUP.
		for _, class := range classes {
			if class == MemoryClass {
				return []ParameterClass{MemoryClass}, nil
			}
		}

		return classes, nil
	}

	return nil, fmt.Errorf("unsupported parameter kind %s", descriptor.Kind)
}

func (descriptor *DataDescriptor) IsNonTrivialForCalls() (bool, error) {
	if descriptor.Kind == ArrayKind {
		return descriptor.Value.IsNonTrivialForCalls()
	} else if descriptor.Kind == StructKind || descriptor.Kind == UnionKind {
		for _, field := range descriptor.Fields {
			isNonTrivial, err := field.Value.IsNonTrivialForCalls()
			if err != nil {
				return false, err
			} else if isNonTrivial {
				return true, nil
			}
		}

		for _, child := range descriptor.DIE.Children {
			if child.Tag == dwarf.DW_TAG_inheritance { // the base class' entry
				baseClassTypeDie, err := child.TypeEntry()
				if err != nil {
					return false, err
				}

				baseClassType, err := descriptor.Pool.GetVariableDescriptor(
					baseClassTypeDie)
				if err != nil {
					return false, err
				}

				isNonTrivial, err := baseClassType.IsNonTrivialForCalls()
				if err != nil {
					return false, err
				} else if isNonTrivial {
					return true, nil
				}
			}

			virtuality, ok := child.Int(dwarf.DW_AT_virtuality)
			if ok && virtuality != dwarf.DW_VIRTUALITY_none { // has virtual method
				return true, nil
			}

			if child.Tag != dwarf.DW_TAG_subprogram {
				continue
			}

			isCopyOrMoveConstructor, err := descriptor.isCopyOrMoveConstructor(child)
			if err != nil {
				return false, err
			}

			isDestructor, err := descriptor.isDestructor(child)
			if err != nil {
				return false, err
			}

			if !isCopyOrMoveConstructor && !isDestructor {
				continue
			}

			defaulted, ok := child.Int(dwarf.DW_AT_defaulted)
			if !ok || defaulted == dwarf.DW_DEFAULTED_in_class {
				return true, nil
			}
		}
	}

	return false, nil
}

func (descriptor *DataDescriptor) isCopyOrMoveConstructor(
	funcDie *dwarf.DebugInfoEntry,
) (
	bool,
	error,
) {
	funcName, _, err := funcDie.Name()
	if err != nil {
		return false, err
	}

	className, _, err := descriptor.DIE.Name()
	if err != nil {
		return false, err
	}

	if funcName != className {
		return false, nil
	}

	paramCount := 0
	for _, child := range funcDie.Children {
		if child.Tag != dwarf.DW_TAG_formal_parameter {
			continue
		}

		if paramCount == 0 {
			paramTypeDie, err := child.TypeEntry()
			if err != nil {
				return false, err
			}

			if paramTypeDie.Tag != dwarf.DW_TAG_pointer_type {
				return false, nil
			}

			paramType, err := descriptor.Pool.GetVariableDescriptor(paramTypeDie)
			if err != nil {
				return false, nil
			}

			if paramType.Value != descriptor {
				return false, nil
			}
		} else if paramCount == 1 {
			paramTypeDie, err := child.TypeEntry()
			if err != nil {
				return false, err
			}

			if paramTypeDie.Tag != dwarf.DW_TAG_reference_type &&
				paramTypeDie.Tag != dwarf.DW_TAG_rvalue_reference_type {

				return false, nil
			}

			paramType, err := descriptor.Pool.GetVariableDescriptor(paramTypeDie)
			if err != nil {
				return false, nil
			}

			if paramType.Value != descriptor {
				return false, nil
			}
		} else {
			return false, nil
		}

		paramCount += 1
	}

	return true, nil
}

func (descriptor *DataDescriptor) isDestructor(
	funcDie *dwarf.DebugInfoEntry,
) (
	bool,
	error,
) {
	name, _, err := funcDie.Name()
	if err != nil {
		return false, err
	}

	return strings.HasPrefix(name, "~"), nil
}

func (descriptor *DataDescriptor) Alignment() int {
	if descriptor.Kind == ArrayKind {
		return descriptor.Value.Alignment()
	} else if descriptor.Kind == StructKind || descriptor.Kind == UnionKind {
		maxAlignment := 0
		for _, field := range descriptor.Fields {
			fieldAlignment := field.Value.Alignment()
			if maxAlignment < fieldAlignment {
				maxAlignment = fieldAlignment
			}
		}
		return maxAlignment
	}
	return descriptor.ByteSize
}

func (descriptor *DataDescriptor) HasUnalignedFields() bool {
	if descriptor.Kind != StructKind && descriptor.Kind != UnionKind {
		return false
	}

	for _, field := range descriptor.Fields {
		fieldAlignment := field.Value.Alignment()
		if field.ByteOffset%fieldAlignment != 0 {
			return true
		}

		if field.Value.HasUnalignedFields() {
			return true
		}
	}

	return false
}

// NOTE: structBitOffset is the field's struct bit offset relative to the outer
// most struct
func classifyField(
	classes []ParameterClass,
	field *FieldDescriptor,
	structBitOffset int,
) error {
	currentBitOffset := structBitOffset + field.ByteOffset*8 + field.BitOffset

	if field.Value.Kind == StructKind || field.Value.Kind == UnionKind {
		for _, subField := range field.Value.Fields {
			classifyField(classes, subField, currentBitOffset)
		}
	} else {
		fieldClasses, err := field.Value.ParameterClasses()
		if err != nil {
			return err
		}

		startIdx := currentBitOffset / 64
		for idx, fieldClass := range fieldClasses {
			currentIdx := startIdx + idx
			classes[currentIdx] = mergeFieldClasses(classes[currentIdx], fieldClass)
		}
	}

	return nil
}

// Merge pairwise field classes using rules 4a - 4f
func mergeFieldClasses(
	class1 ParameterClass,
	class2 ParameterClass,
) ParameterClass {
	if class1 == class2 {
		return class1
	}

	if class1 == NoClass {
		return class2
	}

	if class2 == NoClass {
		return class1
	}

	if class1 == MemoryClass || class2 == MemoryClass {
		return MemoryClass
	}

	if class1 == IntegerClass || class2 == IntegerClass {
		return IntegerClass
	}

	// NOTE: rule 4e is not applicable since we don't support
	// X87/X87UP/COMPLEX_X87 classes

	return SSEClass
}

func (descriptor *DataDescriptor) TypeName() string {
	if descriptor.Kind == PointerKind {
		return "*" + descriptor.Value.TypeName()
	}

	if descriptor.Kind == ArrayKind {
		return fmt.Sprintf(
			"[%d]%s",
			descriptor.NumElements,
			descriptor.Value.TypeName())
	}

	if descriptor.Kind == StructKind || descriptor.Kind == UnionKind {
		if descriptor.Name == "" {
			return fmt.Sprintf("<unnamed %s>", descriptor.Kind)
		}

		return descriptor.Name
	}

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
	case ArrayKind, StructKind, UnionKind,
		FunctionKind, MethodKind,
		MemberPointerKind, VoidKind:
		return false
	default:
		return true
	}
}

func (descriptor *DataDescriptor) Equals(other *DataDescriptor) bool {
	if descriptor == other {
		return true
	}

	switch descriptor.Kind {
	case MemberPointerKind, StructKind, UnionKind, FunctionKind, MethodKind:
		return false
	}

	if descriptor.Kind != other.Kind {
		return false
	}

	if descriptor.ByteSize != other.ByteSize {
		return false
	}

	if descriptor.Value != nil && !descriptor.Value.Equals(other.Value) {
		return false
	}

	if descriptor.NumElements != other.NumElements {
		return false
	}

	return true
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

		valueDesc, err := descriptor.Pool.GetVariableDescriptor(valueDIE)
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

		valueDesc, err := descriptor.Pool.GetVariableDescriptor(valueDIE)
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

			valueDesc, err := descriptor.Pool.GetVariableDescriptor(valueDIE)
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

type Parameter struct {
	*DataDescriptor

	// When Registers is non-empty, the parameter setup in the list of
	// registers. Otherwise, the parameter setup on stack at rsp + StackOffset
	// (reminder: rsp holds the return address).
	Registers   []string
	StackOffset uint64
}

type SignatureDescriptor struct {
	IsMethod bool

	// The number of bytes on stack allocated for the arguments (this excludes
	// the function return address, which is pushed to the top after arguments
	// are setup)
	ParameterStackSize uint64

	// SYS V ABI requires us to save the number of SSE registers used in rax
	// to keep track of varargs.
	NumSSERegistersUsed uint64

	Parameters []*Parameter

	Return *DataDescriptor

	// When true, the return value is a MemoryClass value. The value is malloc-ed
	// before invoking the function, and the value's address is pass to the
	// function via rdi. The function will return the value's address in rax.
	ReturnInMemory bool

	// When non-empty, the return value is in the listed registers. If the list
	// contains multiple elements, we'll malloc copy the data before returning.
	// If the list contains exactly one element, we'll return the value via
	// ImplicitValue.
	//
	// When ReturnInMemory is true, this list is ignored.
	ReturnOnRegisters []string

	DIE *dwarf.DebugInfoEntry
}

func (signature *SignatureDescriptor) AssignStackAndRegisters() error {
	paramIntRegs := []string{
		"rdi",
		"rsi",
		"rdx",
		"rcx",
		"r8",
		"r9",
	}
	paramSSERegs := []string{
		"xmm0",
		"xmm1",
		"xmm2",
		"xmm3",
		"xmm4",
		"xmm5",
		"xmm6",
		"xmm7",
	}

	classes, err := signature.Return.ParameterClasses()
	if err != nil {
		return fmt.Errorf("return value: %w", err)
	}

	if len(classes) == 1 && classes[0] == MemoryClass {
		signature.ReturnInMemory = true
		paramIntRegs = paramIntRegs[1:] // return value's address is in rdi
	} else {
		retIntRegs := []string{"rax", "rdx"}
		retSSERegs := []string{"xmm0", "xmm1"}

		for _, class := range classes {
			if class == IntegerClass {
				signature.ReturnOnRegisters = append(
					signature.ReturnOnRegisters,
					retIntRegs[0])
				retIntRegs = retIntRegs[1:]
			} else if class == SSEClass {
				signature.ReturnOnRegisters = append(
					signature.ReturnOnRegisters,
					retSSERegs[0])
				retSSERegs = retSSERegs[1:]
			} else {
				panic("should never happen")
			}
		}
	}

	maybeAllocateRegisters := func(classes []ParameterClass) []string {
		if len(classes) == 1 && classes[0] == MemoryClass {
			return nil
		}

		numInts := 0
		numSSEs := 0
		for _, class := range classes {
			if class == IntegerClass {
				numInts += 1
			} else if class == SSEClass {
				numSSEs += 1
			} else {
				panic("should never happen")
			}
		}

		if len(paramIntRegs) < numInts || len(paramSSERegs) < numSSEs {
			return nil
		}

		regs := []string{}
		for _, class := range classes {
			if class == IntegerClass {
				regs = append(regs, paramIntRegs[0])
				paramIntRegs = paramIntRegs[1:]
			} else {
				regs = append(regs, paramSSERegs[0])
				paramSSERegs = paramSSERegs[1:]
			}
		}

		return regs
	}

	paramStackSize := 0
	for idx, param := range signature.Parameters {
		classes, err := param.ParameterClasses()
		if err != nil {
			return fmt.Errorf("parameter %d: %w", idx, err)
		}

		registers := maybeAllocateRegisters(classes)
		if len(registers) == 0 { // in memory
			param.StackOffset = uint64(8 + paramStackSize)
			paramStackSize += param.ByteSize
		} else {
			param.Registers = registers
		}
	}

	signature.ParameterStackSize = uint64(paramStackSize)
	signature.NumSSERegistersUsed = uint64(8 - len(paramSSERegs))

	return nil
}

func (signature *SignatureDescriptor) Matches(arguments []*TypedData) bool {
	parameters := signature.Parameters
	if signature.IsMethod {
		parameters = parameters[1:]
	}

	if len(parameters) != len(arguments) {
		return false
	}

	for idx, paramType := range parameters {
		if !paramType.Equals(arguments[idx].DataDescriptor) {
			return false
		}
	}

	return true
}

type methodKey struct {
	receiverTypeDie *dwarf.DebugInfoEntry
	name            string
}

type unboundMethod struct {
	*DataDescriptor
	Addresses []VirtualAddress
}

type DataDescriptorPool struct {
	loadedElves *loadedelves.Files
	memory      *memory.VirtualMemory

	// For non-function kinds
	variableDescriptors map[*dwarf.DebugInfoEntry]*DataDescriptor

	functions map[string]*TypedData

	methods map[methodKey]unboundMethod
}

func NewDataDescriptorPool(
	loadedElves *loadedelves.Files,
	mem *memory.VirtualMemory,
) *DataDescriptorPool {
	return &DataDescriptorPool{
		loadedElves:         loadedElves,
		memory:              mem,
		variableDescriptors: map[*dwarf.DebugInfoEntry]*DataDescriptor{},
		functions:           map[string]*TypedData{},
		methods:             map[methodKey]unboundMethod{},
	}
}

func (pool *DataDescriptorPool) GetVariableDescriptor(
	typeDie *dwarf.DebugInfoEntry,
) (
	*DataDescriptor,
	error,
) {
	descriptor, ok := pool.variableDescriptors[typeDie]
	if ok {
		return descriptor, nil
	}

	descriptor, err := pool.parseDataTypeDIE(typeDie)
	if err != nil {
		return nil, err
	}

	// Insert parsed descriptor into pool so that self-type pointer references
	// can be resolved.
	pool.variableDescriptors[typeDie] = descriptor

	err = descriptor.resolveSizeAndValueDescriptor()
	if err != nil {
		return nil, err
	}

	return descriptor, nil
}

func (pool *DataDescriptorPool) parseDataTypeDIE(
	die *dwarf.DebugInfoEntry,
) (
	*DataDescriptor,
	error,
) {
	switch die.Tag {
	case dwarf.DW_TAG_base_type:
		return pool.parseBaseType(die)

	case dwarf.DW_TAG_pointer_type, dwarf.DW_TAG_reference_type:
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

		return pool.GetVariableDescriptor(base)
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
			// NOTE: x87 long double aren't supported.
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

		location, ok := child.Uint(dwarf.DW_AT_data_member_location)
		if !ok { // static field member
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

	name, _, err := die.Name()
	if err != nil {
		return nil, fmt.Errorf("struct name error: %w", err)
	}

	// NOTE: field value descriptors are defer resolved.
	return &DataDescriptor{
		Pool:     pool,
		Kind:     kind,
		ByteSize: int(byteSize),
		Name:     name,
		Fields:   fields,
		DIE:      die,
	}, nil
}

func (pool *DataDescriptorPool) NewVoidType() *DataDescriptor {
	return &DataDescriptor{
		Pool:     pool,
		Kind:     VoidKind,
		ByteSize: 0,
	}
}

func (pool *DataDescriptorPool) NewVoid() *TypedData {
	return &TypedData{
		VirtualMemory:  pool.memory,
		FormatPrefix:   "(void)",
		DataDescriptor: pool.NewVoidType(),
	}
}

func (pool *DataDescriptorPool) NewBoolType() *DataDescriptor {
	return &DataDescriptor{
		Pool:     pool,
		Kind:     BoolKind,
		ByteSize: 1,
	}
}

func (pool *DataDescriptorPool) NewBool(value bool) *TypedData {
	return &TypedData{
		VirtualMemory:  pool.memory,
		FormatPrefix:   fmt.Sprintf("%v", value),
		DataDescriptor: pool.NewBoolType(),
		ImplicitValue:  value,
	}
}

func (pool *DataDescriptorPool) NewCharType() *DataDescriptor {
	return &DataDescriptor{
		Pool:     pool,
		Kind:     CharKind,
		ByteSize: 1,
	}
}

func (pool *DataDescriptorPool) NewChar(
	formatPrefix string,
	value byte,
) *TypedData {
	return &TypedData{
		VirtualMemory:  pool.memory,
		FormatPrefix:   formatPrefix,
		DataDescriptor: pool.NewCharType(),
		ImplicitValue:  value,
	}
}

func (pool *DataDescriptorPool) NewInt32Type() *DataDescriptor {
	return &DataDescriptor{
		Pool:     pool,
		Kind:     IntKind,
		ByteSize: 4,
	}
}

func (pool *DataDescriptorPool) NewInt32(
	formatPrefix string,
	value int32,
) *TypedData {
	return &TypedData{
		VirtualMemory:  pool.memory,
		FormatPrefix:   formatPrefix,
		DataDescriptor: pool.NewInt32Type(),
		ImplicitValue:  value,
	}
}

func (pool *DataDescriptorPool) NewInt64Type() *DataDescriptor {
	return &DataDescriptor{
		Pool:     pool,
		Kind:     IntKind,
		ByteSize: 8,
	}
}

func (pool *DataDescriptorPool) NewInt64(
	formatPrefix string,
	value int64,
) *TypedData {
	return &TypedData{
		VirtualMemory:  pool.memory,
		FormatPrefix:   formatPrefix,
		DataDescriptor: pool.NewInt64Type(),
		ImplicitValue:  value,
	}
}

func (pool *DataDescriptorPool) NewFloat64Type() *DataDescriptor {
	return &DataDescriptor{
		Pool:     pool,
		Kind:     FloatKind,
		ByteSize: 8,
	}
}

func (pool *DataDescriptorPool) NewFloat64(
	formatPrefix string,
	value float64,
) *TypedData {
	return &TypedData{
		VirtualMemory:  pool.memory,
		FormatPrefix:   formatPrefix,
		DataDescriptor: pool.NewFloat64Type(),
		ImplicitValue:  value,
	}
}

func (pool *DataDescriptorPool) NewPointerType(
	valueType *DataDescriptor,
) *DataDescriptor {
	return &DataDescriptor{
		Pool:     pool,
		Kind:     PointerKind,
		ByteSize: 8,
		Value:    valueType,
		resolved: true,
	}
}

func (pool *DataDescriptorPool) NewCString(
	context EvaluationContext,
	formatPrefix string,
	value string,
) (
	*TypedData,
	error,
) {
	data := append([]byte(value), 0) // append \0 to terminate string

	address, err := context.InvokeMallocInCurrentThread(len(data))
	if err != nil {
		return nil, fmt.Errorf("cannot allocate c string: %w", err)
	}

	n, err := pool.memory.Write(address, data)
	if err != nil {
		return nil, fmt.Errorf("failed to copy c string: %w", err)
	}
	if n != len(data) {
		return nil, fmt.Errorf("failed to copy all c string data")
	}

	return &TypedData{
		VirtualMemory:  pool.memory,
		FormatPrefix:   formatPrefix,
		DataDescriptor: pool.NewPointerType(pool.NewCharType()),
		ImplicitValue:  address,
	}, nil
}

func (pool *DataDescriptorPool) GetMalloc() (*TypedData, error) {
	function, ok := pool.functions["malloc"]
	if ok {
		return function, nil
	}

	// NOTE: We assume that libc was not build in debug mode and hence does not
	// include dwarf DIE for malloc.
	//
	// There could be multiple malloc symbol entries, most of which will have
	// zero address value.
	var mallocAddress VirtualAddress
	for _, symbol := range pool.loadedElves.SymbolsByName("malloc") {
		if symbol.Type() == elf.SymbolTypeFunction && symbol.Value != 0 {
			addr, err := pool.loadedElves.SymbolToVirtualAddress(symbol)
			if err != nil {
				return nil, err
			}

			mallocAddress = addr
			break
		}
	}

	if mallocAddress == 0 {
		return nil, fmt.Errorf("malloc not found")
	}

	signature := &SignatureDescriptor{
		Parameters: []*Parameter{
			&Parameter{
				DataDescriptor: pool.NewInt32Type(),
			},
		},
		// NOTE: The exact value type doesn't matter. We only care about the
		// address.
		Return: pool.NewPointerType(pool.NewCharType()),
	}

	err := signature.AssignStackAndRegisters()
	if err != nil {
		return nil, err
	}

	descriptor := &DataDescriptor{
		Pool:     pool,
		Kind:     FunctionKind,
		ByteSize: 8, // size of virtual address
		Name:     "malloc",
		Signatures: []*SignatureDescriptor{
			signature,
		},
		resolved: true,
	}

	function = &TypedData{
		VirtualMemory:  pool.memory,
		FormatPrefix:   "malloc",
		DataDescriptor: descriptor,
		FunctionAddresses: []VirtualAddress{
			mallocAddress,
		},
	}

	pool.functions["malloc"] = function
	return function, nil
}

func (pool *DataDescriptorPool) GetFunction(
	name string,
) (
	*TypedData,
	error,
) {
	function, ok := pool.functions[name]
	if ok {
		return function, nil
	}

	functionDefs, err := pool.loadedElves.FunctionDefinitionEntriesWithName(name)
	if err != nil {
		return nil, err
	}

	if len(functionDefs) == 0 {
		return nil, nil
	}

	signatures, addresses, err := pool.parseSignatures(false, functionDefs)
	if err != nil {
		return nil, err
	}

	descriptor := &DataDescriptor{
		Pool:       pool,
		Kind:       FunctionKind,
		ByteSize:   8, // size of virtual address
		Name:       name,
		Signatures: signatures,
		resolved:   true,
	}

	function = &TypedData{
		VirtualMemory:     pool.memory,
		FormatPrefix:      name,
		DataDescriptor:    descriptor,
		FunctionAddresses: addresses,
	}

	pool.functions[name] = function
	return function, nil
}

func (pool *DataDescriptorPool) GetMethod(
	receiverTypeDie *dwarf.DebugInfoEntry,
	methodName string,
) (
	*DataDescriptor,
	[]VirtualAddress,
	error,
) {
	key := methodKey{
		receiverTypeDie: receiverTypeDie,
		name:            methodName,
	}

	method, ok := pool.methods[key]
	if ok {
		return method.DataDescriptor, method.Addresses, nil
	}

	methodDefs := []*dwarf.DebugInfoEntry{}
	for _, child := range receiverTypeDie.Children {
		if child.Tag != dwarf.DW_TAG_subprogram {
			continue
		}

		if child.SpecIndex(dwarf.DW_AT_object_pointer) == -1 {
			continue
		}

		name, ok, err := child.Name()
		if err != nil {
			return nil, nil, err
		}

		if !ok || name != methodName {
			continue
		}

		methodDef, err := child.FindMethodDefinitionEntry()
		if err != nil {
			return nil, nil, err
		}

		methodDefs = append(methodDefs, methodDef)
	}

	if len(methodDefs) == 0 {
		return nil, nil, nil
	}

	receiverDescriptor, err := pool.GetVariableDescriptor(receiverTypeDie)
	if err != nil {
		return nil, nil, err
	}

	signatures, addresses, err := pool.parseSignatures(true, methodDefs)
	if err != nil {
		return nil, nil, err
	}

	descriptor := &DataDescriptor{
		Pool:       pool,
		Kind:       MethodKind,
		ByteSize:   8, // size of virtual address
		Name:       methodName,
		Value:      receiverDescriptor,
		Signatures: signatures,
		resolved:   true,
	}

	pool.methods[key] = unboundMethod{
		DataDescriptor: descriptor,
		Addresses:      addresses,
	}
	return descriptor, addresses, nil
}

func (pool *DataDescriptorPool) parseSignatures(
	isMethod bool,
	functionDies []*dwarf.DebugInfoEntry,
) (
	[]*SignatureDescriptor,
	[]VirtualAddress,
	error,
) {
	signatures := []*SignatureDescriptor{}
	addresses := []VirtualAddress{}
	for _, funcDie := range functionDies {
		parameters := []*Parameter{}
		for _, child := range funcDie.Children {
			if child.Tag != dwarf.DW_TAG_formal_parameter {
				continue
			}

			paramTypeDie, err := child.TypeEntry()
			if err != nil {
				return nil, nil, fmt.Errorf("parameter type error: %w", err)
			}

			paramDescriptor, err := pool.GetVariableDescriptor(paramTypeDie)
			if err != nil {
				return nil, nil, err
			}

			parameters = append(
				parameters,
				&Parameter{
					DataDescriptor: paramDescriptor,
				})
		}

		var retDescriptor *DataDescriptor
		if funcDie.SpecIndex(dwarf.DW_AT_type) != -1 { // has return value

			retTypeDie, err := funcDie.TypeEntry()
			if err != nil {
				return nil, nil, fmt.Errorf("return type error: %w", err)
			}

			retDescriptor, err = pool.GetVariableDescriptor(retTypeDie)
			if err != nil {
				return nil, nil, err
			}
		} else {
			retDescriptor = pool.NewVoidType()
		}

		addressRanges, err := pool.loadedElves.ToVirtualAddressRanges(funcDie)
		if err != nil {
			return nil, nil, err
		}

		signature := &SignatureDescriptor{
			IsMethod:   isMethod,
			Parameters: parameters,
			Return:     retDescriptor,
			DIE:        funcDie,
		}

		err = signature.AssignStackAndRegisters()
		if err != nil {
			return nil, nil, err
		}

		signatures = append(signatures, signature)
		addresses = append(addresses, addressRanges[0].Low)
	}

	return signatures, addresses, nil
}
