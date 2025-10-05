package registers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pattyshack/bad/dwarf"
)

// The register class determines where the register data is located:
// - GeneralRegister -> user::regs (user_regs_struct)
// - FloatingPointClass -> user:i387 (user_fpregs_struct)
// - DebugClass -> user::u_debugreg ([8]uint64)
type Class string

const (
	GeneralClass       = Class("general")
	FloatingPointClass = Class("floating point")
	DebugClass         = Class("debug")

	stSpace   = "StSpace"
	xmmSpace  = "XmmSpace"
	uDebugReg = "UDebugReg"
)

type Spec struct {
	SortId int

	Name             string
	dwarf.RegisterId // -1 for invalid

	Size uintptr // register size in bytes

	Class Class

	// Only applicable to general / floating point registers
	Field string

	// Only applicable to 8-bit general register (ah/bh/ch/dh)
	IsHighRegister bool

	// Only applicable to st / mm / xmm / debug registers.
	Index int
}

// Valid types:
//
// 8-bit register: Uint8, Int8
// 16-bit register: Uint16, Int16
// 32-bit register: Uint32, Int32
// 64-bit register: Uint64, Int64
// 128-bit (floating point) register: Uint128, Float32, Float64
//
// uint and float are zero extended, int is sign extended.
//
// NOTE: mm0, ..., mm7 are in reality 8-byte registers, and st0, ..., st7 are
// in reality 10-byte registers, but both have 16-byte representation in linux.
func (reg Spec) CanAccept(value Value) error {
	// dr4 and dr5 are not real registers
	// https://en.wikipedia.org/wiki/X86_debug_register
	if reg.Class == DebugClass && (reg.Index == 4 || reg.Index == 5) {
		return fmt.Errorf("cannot set %s.  register is read-only", reg.Name)
	}

	// 128-bit floating point registers are special cased
	if reg.Class == FloatingPointClass && reg.Size == 16 {
		if value.IsFloat() {
			return nil
		}

		_, ok := value.(Uint128)
		if ok {
			return nil
		}

		return fmt.Errorf(
			"register (%s) expects Uint128/Float32/Float64 value. found %#v",
			reg.Name,
			value)
	}

	// All other registers

	if value.IsFloat() {
		return fmt.Errorf(
			"cannot use floating point value in register (%s)",
			reg.Name)
	}

	if reg.Size != value.Size() {
		return fmt.Errorf(
			"register (%s) size (%d) does not match value size (%d)",
			reg.Name,
			reg.Size,
			value.Size())
	}

	return nil
}

func (reg Spec) ParseValue(value string) (Value, error) {
	if strings.HasPrefix(value, "f:") {
		floatValue, err := strconv.ParseFloat(value[2:], 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse float32 (%s): %w", value[2:], err)
		}

		return F32(float32(floatValue)), nil
	} else if strings.HasPrefix(value, "d:") {
		floatValue, err := strconv.ParseFloat(value[2:], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse float64 (%s): %w", value[2:], err)
		}

		return F64(floatValue), nil
	} else if strings.HasPrefix(value, "i:") {
		bitSize := int(reg.Size * 8)
		if bitSize > 64 {
			bitSize = 64
		}
		intValue, err := strconv.ParseInt(value[2:], 0, bitSize)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int (%s): %w", value[2:], err)
		}

		switch reg.Size {
		case 1:
			return I8(int8(intValue)), nil
		case 2:
			return I16(int16(intValue)), nil
		case 4:
			return I32(int32(intValue)), nil
		case 8, 16:
			return I64(intValue), nil
		default:
			panic(fmt.Sprintf("unhandled size %d", reg.Size))
		}
	}

	chunks := strings.Split(value, ":")
	if len(chunks) == 2 { // u128
		high, err := strconv.ParseUint(chunks[0], 0, 64)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse uint128 high word (%s): %w",
				chunks[0],
				err)
		}

		low, err := strconv.ParseUint(chunks[1], 0, 64)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse uint128 low word (%s): %w",
				chunks[1],
				err)
		}

		return U128(high, low), nil
	}

	bitSize := int(reg.Size * 8)
	if bitSize > 64 {
		bitSize = 64
	}

	uintValue, err := strconv.ParseUint(value, 0, bitSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse uint (%s): %w", value, err)
	}

	switch reg.Size {
	case 1:
		return U8(uint8(uintValue)), nil
	case 2:
		return U16(uint16(uintValue)), nil
	case 4:
		return U32(uint32(uintValue)), nil
	case 8, 16:
		return U64(uintValue), nil
	default:
		panic(fmt.Sprintf("unhandled size %d", reg.Size))
	}
}

var (
	OrderedSpecs []Spec
	NameSpecs    map[string]Spec           = map[string]Spec{}
	IdSpecs      map[dwarf.RegisterId]Spec = map[dwarf.RegisterId]Spec{}

	ProgramCounter Spec
	StackPointer   Spec
	FramePointer   Spec

	DebugControl   Spec
	DebugStatus    Spec
	DebugAddresses []Spec

	SyscallNum  Spec
	SyscallArgs []Spec
	SyscallRet  Spec
)

func ByName(name string) (Spec, bool) {
	reg, ok := NameSpecs[name]
	return reg, ok
}

func ById(id dwarf.RegisterId) (Spec, bool) {
	reg, ok := IdSpecs[id]
	return reg, ok
}

func init() {
	nextId := 0

	addRegister := func(
		name string,
		dwarfId int,
		size uintptr,
		class Class,
		field string,
		isHigh bool,
		index int,
	) {
		entry := Spec{
			SortId:         nextId,
			Name:           name,
			RegisterId:     dwarf.RegisterId(dwarfId),
			Size:           size,
			Class:          class,
			Field:          field,
			IsHighRegister: isHigh,
			Index:          index,
		}
		nextId += 1

		OrderedSpecs = append(OrderedSpecs, entry)

		_, ok := NameSpecs[name]
		if ok {
			panic("duplicate register info: " + name)
		}
		NameSpecs[name] = entry

		if entry.RegisterId != -1 {
			_, ok := IdSpecs[entry.RegisterId]
			if ok {
				panic("duplicate register info: " + name)
			}
			IdSpecs[entry.RegisterId] = entry
		}
	}

	addGpr64 := func(name string, dwarfId int, field string) {
		addRegister(name, dwarfId, 8, GeneralClass, field, false, 0)
	}

	addSubGpr32 := func(name string, field string) {
		addRegister(name, -1, 4, GeneralClass, field, false, 0)
	}

	addSubGpr16 := func(name string, field string) {
		addRegister(name, -1, 2, GeneralClass, field, false, 0)
	}

	addSubGpr8 := func(name string, field string, isHigh bool) {
		addRegister(name, -1, 1, GeneralClass, field, isHigh, 0)
	}

	addFpr16 := func(name string, dwarfId int, field string) {
		addRegister(name, dwarfId, 2, FloatingPointClass, field, false, 0)
	}

	addFpr32 := func(name string, dwarfId int, field string) {
		addRegister(name, dwarfId, 4, FloatingPointClass, field, false, 0)
	}

	addFpr64 := func(name string, field string) {
		addRegister(name, -1, 8, FloatingPointClass, field, false, 0)
	}

	addFpr128 := func(
		prefix string,
		dwarfIdStart int,
		field string,
		idx int,
	) {
		addRegister(
			fmt.Sprintf("%s%d", prefix, idx),
			dwarfIdStart+idx,
			16,
			FloatingPointClass,
			field,
			false,
			idx)
	}

	addDr64 := func(idx int) {
		addRegister(
			fmt.Sprintf("dr%d", idx),
			-1,
			8,
			DebugClass,
			uDebugReg,
			false,
			idx)
	}

	dwarfIds := map[string]int{
		"rip":    16,
		"eflags": 49,
		"cs":     51,
		"fs":     54,
		"gs":     55,
		"ss":     52,
		"ds":     53,
		"es":     50,
	}

	names := strings.Split(
		"rax rdx rcx rbx rsi rdi rbp rsp "+
			"r8 r9 r10 r11 r12 r13 r14 r15 "+
			"rip eflags cs fs gs ss ds es",
		" ")
	for idx, name := range names {
		dwarfId, ok := dwarfIds[name]
		if !ok {
			dwarfId = idx
		}

		field := strings.ToUpper(name[0:1]) + name[1:]

		addGpr64(name, dwarfId, field)

		if ok { // not general compute registers
			continue
		} else if strings.ContainsAny(name, "189") { // newer x64 registers
			addSubGpr32(name+"d", field)
			addSubGpr16(name+"w", field)
			addSubGpr8(name+"b", field, false)
		} else { // legacy x86 extended registers
			addSubGpr32("e"+name[1:], field)
			addSubGpr16(name[1:], field)

			if name[2] == 'x' {
				prefix := name[1:2]
				addSubGpr8(prefix+"h", field, true)
				addSubGpr8(prefix+"l", field, false)
			} else {
				addSubGpr8(name[1:]+"l", field, false)
			}
		}
	}

	addGpr64("orig_rax", -1, "Orig_rax")

	addFpr16("fcw", 65, "Cwd")
	addFpr16("fsw", 66, "Swd")
	addFpr16("ftw", -1, "Ftw")
	addFpr16("fop", -1, "Fop")
	addFpr64("frip", "Rip")
	addFpr64("frdp", "Rdp")
	addFpr32("mxcsr", 64, "Mxcsr")
	addFpr32("mxcrmask", -1, "MxcrMask")

	for i := 0; i < 8; i++ {
		addFpr128("st", 33, stSpace, i) // st0, ..., st7
		addFpr128("mm", 41, stSpace, i) // mm0, ..., mm7
	}

	for i := 0; i < 16; i++ { // xmm0, ..., xmm15
		addFpr128("xmm", 17, xmmSpace, i)
	}

	for i := 0; i < 8; i++ {
		addDr64(i)
	}

	ProgramCounter, _ = ByName("rip")
	StackPointer, _ = ByName("rsp")
	FramePointer, _ = ByName("rbp")

	DebugControl, _ = ByName("dr7")
	DebugStatus, _ = ByName("dr6")

	for _, name := range []string{"dr0", "dr1", "dr2", "dr3"} {
		reg, _ := ByName(name)
		DebugAddresses = append(DebugAddresses, reg)
	}

	SyscallNum, _ = ByName("orig_rax")
	SyscallRet, _ = ByName("rax")
	for _, arg := range []string{"rdi", "rsi", "rdx", "r10", "r8", "r9"} {
		reg, _ := ByName(arg)
		SyscallArgs = append(SyscallArgs, reg)
	}
}
