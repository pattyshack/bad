package bad

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"unsafe"

	"github.com/pattyshack/bad/ptrace"
)

// The register class determines where the register data is located:
// - GeneralRegister -> user::regs (user_regs_struct)
// - FloatingPointRegister -> user:i387 (user_fpregs_struct)
// - DebugRegister -> user::u_debugreg ([8]uint64)
type RegisterClass string

const (
	GeneralRegister       = RegisterClass("general")
	FloatingPointRegister = RegisterClass("floating point")
	DebugRegister         = RegisterClass("debug")

	stSpace   = "StSpace"
	xmmSpace  = "XmmSpace"
	uDebugReg = "UDebugReg"
)

// Valid types:
//
// 8-bit register: Uint[8], Int[8]
// 16-bit register: Uint[16], Int[16]
// 32-bit register: Uint[32], Int[32]
// 64-bit register: Uint[64], Int[64]
// 128-bit (floating point) register: [2]uint64, Float32, Float64
//
// uint and float are zero extended, int is sign extended.
type RegisterValue interface {
	Size() uintptr
	IsFloat() bool

	ToUint32() uint32
	ToUint64() uint64
	ToUint128() Uint128
}

type Uint128 struct {
	Low  uint64
	High uint64
}

func (Uint128) Size() uintptr {
	return 16
}

func (Uint128) IsFloat() bool {
	return false
}

func (u Uint128) ToUint32() uint32 {
	return uint32(u.Low)
}

func (u Uint128) ToUint64() uint64 {
	return u.Low
}

func (u Uint128) ToUint128() Uint128 {
	return u
}

func Uint128Value(low uint64, high uint64) Uint128 {
	return Uint128{
		Low:  low,
		High: high,
	}
}

type Uint[T uint8 | uint16 | uint32 | uint64] struct {
	Value T
}

func (u Uint[T]) Size() uintptr {
	return unsafe.Sizeof(u.Value)
}

func (Uint[T]) IsFloat() bool {
	return false
}

func (u Uint[T]) ToUint32() uint32 {
	return uint32(u.Value)
}

func (u Uint[T]) ToUint64() uint64 {
	return uint64(u.Value)
}

func (u Uint[T]) ToUint128() Uint128 {
	return Uint128{
		Low:  uint64(u.Value),
		High: 0,
	}
}

type Uint8 = Uint[uint8]

func Uint8Value(v uint8) RegisterValue {
	return Uint8{
		Value: v,
	}
}

type Uint16 = Uint[uint16]

func Uint16Value(v uint16) RegisterValue {
	return Uint16{
		Value: v,
	}
}

type Uint32 = Uint[uint32]

func Uint32Value(v uint32) RegisterValue {
	return Uint32{
		Value: v,
	}
}

type Uint64 = Uint[uint64]

func Uint64Value(v uint64) RegisterValue {
	return Uint64{
		Value: v,
	}
}

type Int[T int8 | int16 | int32 | int64] struct {
	Value T
}

func (i Int[T]) Size() uintptr {
	return unsafe.Sizeof(i.Value)
}

func (Int[T]) IsFloat() bool {
	return false
}

func (i Int[T]) ToUint32() uint32 {
	return uint32(int64(i.Value))
}

func (i Int[T]) ToUint64() uint64 {
	return uint64(int64(i.Value))
}

func (i Int[T]) ToUint128() Uint128 {
	low := i.ToUint64()
	high := uint64(0) // positive sign extended
	if i.Value < 0 {
		high = ^high // negative sign extended
	}
	return Uint128Value(low, high)
}

type Int8 = Int[int8]

func Int8Value(v int8) RegisterValue {
	return Int[int8]{
		Value: v,
	}
}

type Int16 = Int[int16]

func Int16Value(v int16) RegisterValue {
	return Int[int16]{
		Value: v,
	}
}

type Int32 = Int[int32]

func Int32Value(v int32) RegisterValue {
	return Int[int32]{
		Value: v,
	}
}

type Int64 = Int[int64]

func Int64Value(v int64) RegisterValue {
	return Int[int64]{
		Value: v,
	}
}

type Float32 float32

func (Float32) Size() uintptr {
	return 4
}

func (Float32) IsFloat() bool {
	return true
}

func (f Float32) ToUint32() uint32 {
	return math.Float32bits(float32(f))
}

func (f Float32) ToUint64() uint64 {
	return uint64(f.ToUint32())
}

func (f Float32) ToUint128() Uint128 {
	return Uint128Value(f.ToUint64(), 0)
}

func Float32Value(v float32) RegisterValue {
	return Float32(v)
}

type Float64 float64

func (Float64) Size() uintptr {
	return 8
}

func (Float64) IsFloat() bool {
	return true
}

func (f Float64) ToUint32() uint32 {
	return uint32(f.ToUint64())
}

func (f Float64) ToUint64() uint64 {
	return math.Float64bits(float64(f))
}

func (f Float64) ToUint128() Uint128 {
	return Uint128Value(f.ToUint64(), 0)
}

func Float64Value(v float64) RegisterValue {
	return Float64(v)
}

type RegisterInfo struct {
	Name    string
	DwarfId int // -1 for invalid

	Size uintptr // register size in bytes

	Class RegisterClass

	// Only applicable to general / floating point registers
	Field string

	// Only applicable to 8-bit general register (ah/bh/ch/dh)
	IsHighRegister bool

	// Only applicable to st / mm / xmm / debug registers.
	Index int
}

// Valid types:
//
// 8-bit register: uint8, int8
// 16-bit register: uint16, int16
// 32-bit register: uint32, int32
// 64-bit register: uint64, int64
// 128-bit (floating point) register: [2]uint64, float32, float64
//
// uint and float are zero extended, int is sign extended.
func (reg RegisterInfo) CanAccept(value RegisterValue) error {
	// dr4 and dr5 are not real registers
	// https://en.wikipedia.org/wiki/X86_debug_register
	if reg.Class == DebugRegister && (reg.Index == 4 || reg.Index == 5) {
		return fmt.Errorf("cannot set %s.  register is read-only", reg.Name)
	}

	// 128-bit floating point registers are special cased
	if reg.Class == FloatingPointRegister && reg.Size == 16 {
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

	if reg.Size != value.Size() {
		return fmt.Errorf(
			"register (%s) size (%d) does not match value size (%d)",
			reg.Name,
			reg.Size,
			value.Size())
	}

	if value.IsFloat() {
		return fmt.Errorf(
			"cannot use floating point value in register (%s)",
			reg.Name)
	}

	return nil
}

type RegisterState struct {
	gpr ptrace.UserRegs
	fpr ptrace.UserFPRegs
	dr  [8]uintptr
}

// This always returns Uint8 / Uint16 / Uint32 / Uint64 / Uint128 depending on
// the register size.
func (state RegisterState) Value(reg RegisterInfo) RegisterValue {
	var data reflect.Value
	switch reg.Class {
	case GeneralRegister:
		data = reflect.ValueOf(state.gpr)
	case FloatingPointRegister:
		if reg.Field == stSpace {
			return Uint128Value(
				state.fpr.StSpace[2*reg.Index],
				state.fpr.StSpace[2*reg.Index+1])
		}

		if reg.Field == xmmSpace {
			return Uint128Value(
				state.fpr.XmmSpace[2*reg.Index],
				state.fpr.XmmSpace[2*reg.Index+1])
		}

		data = reflect.ValueOf(state.fpr)
	case DebugRegister:
		return Uint64Value(uint64(state.dr[reg.Index]))
	default:
		panic(fmt.Sprintf("invalid register: %#v", reg))
	}

	value := data.FieldByName(reg.Field).Uint()
	switch reg.Size {
	case 1:
		if reg.IsHighRegister {
			value >>= 8
		}

		return Uint8Value(uint8(value))
	case 2:
		return Uint16Value(uint16(value))
	case 4:
		return Uint32Value(uint32(value))
	case 8:
		return Uint64Value(value)
	default:
		panic(fmt.Sprintf("invalid register: %#v", reg))
	}
}

func (state RegisterState) WithValue(
	reg RegisterInfo,
	value RegisterValue,
) (
	RegisterState,
	error,
) {
	err := reg.CanAccept(value)
	if err != nil {
		return RegisterState{}, err
	}

	newState := state

	var data reflect.Value
	switch reg.Class {
	case GeneralRegister:
		data = reflect.Indirect(reflect.ValueOf(&newState.gpr))
	case FloatingPointRegister:
		if reg.Field == stSpace {
			u128 := value.ToUint128()

			newState.fpr.StSpace[2*reg.Index] = u128.Low
			newState.fpr.StSpace[2*reg.Index+1] = u128.High

			return newState, nil
		}

		if reg.Field == xmmSpace {
			u128 := value.ToUint128()

			newState.fpr.XmmSpace[2*reg.Index] = u128.Low
			newState.fpr.XmmSpace[2*reg.Index+1] = u128.High

			return newState, nil
		}

		data = reflect.Indirect(reflect.ValueOf(&newState.fpr))
	case DebugRegister:
		newState.dr[reg.Index] = uintptr(value.ToUint64())

		return newState, nil
	default:
		panic(fmt.Sprintf("invalid register: %#v", reg))
	}

	val := value.ToUint64()
	if reg.IsHighRegister {
		val <<= 8
	}

	data.FieldByName(reg.Field).SetUint(val)
	return newState, nil
}

type RegisterSet map[string]RegisterInfo

func (set RegisterSet) addRegister(
	name string,
	dwarfId int,
	size uintptr,
	class RegisterClass,
	field string,
	isHigh bool,
	index int,
) {
	_, ok := set[name]
	if ok {
		panic("duplicate register info: " + name)
	}

	set[name] = RegisterInfo{
		Name:           name,
		DwarfId:        dwarfId,
		Size:           size,
		Class:          class,
		Field:          field,
		IsHighRegister: isHigh,
		Index:          index,
	}
}

func (set RegisterSet) addGpr64(name string, dwarfId int, field string) {
	set.addRegister(name, dwarfId, 8, GeneralRegister, field, false, 0)
}

func (set RegisterSet) addSubGpr32(name string, field string) {
	set.addRegister(name, -1, 4, GeneralRegister, field, false, 0)
}

func (set RegisterSet) addSubGpr16(name string, field string) {
	set.addRegister(name, -1, 2, GeneralRegister, field, false, 0)
}

func (set RegisterSet) addSubGpr8(name string, field string, isHigh bool) {
	set.addRegister(name, -1, 1, GeneralRegister, field, isHigh, 0)
}

func (set RegisterSet) addFpr16(name string, dwarfId int, field string) {
	set.addRegister(name, dwarfId, 2, FloatingPointRegister, field, false, 0)
}

func (set RegisterSet) addFpr32(name string, dwarfId int, field string) {
	set.addRegister(name, dwarfId, 4, FloatingPointRegister, field, false, 0)
}

func (set RegisterSet) addFpr64(name string, field string) {
	set.addRegister(name, -1, 8, FloatingPointRegister, field, false, 0)
}

func (set RegisterSet) addFpr128(
	prefix string,
	dwarfIdStart int,
	field string,
	idx int,
) {
	set.addRegister(
		fmt.Sprintf("%s%d", prefix, idx),
		dwarfIdStart+idx,
		16,
		FloatingPointRegister,
		field,
		false,
		idx)
}

func (set RegisterSet) addDr64(idx int) {
	set.addRegister(
		fmt.Sprintf("dr%d", idx),
		-1,
		8,
		DebugRegister,
		uDebugReg,
		false,
		idx)
}

func NewRegisterSet() RegisterSet {
	set := RegisterSet{}

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

		set.addGpr64(name, dwarfId, field)

		if ok { // not general compute registers
			continue
		} else if strings.ContainsAny(name, "189") { // newer x64 registers
			set.addSubGpr32(name+"d", field)
			set.addSubGpr16(name+"w", field)
			set.addSubGpr8(name+"b", field, false)
		} else { // legacy x86 extended registers
			set.addSubGpr32("e"+name[1:], field)
			set.addSubGpr16(name[1:], field)

			if name[2] == 'x' {
				prefix := name[1:2]
				set.addSubGpr8(prefix+"h", field, true)
				set.addSubGpr8(prefix+"l", field, false)
			} else {
				set.addSubGpr8(name[1:]+"l", field, false)
			}
		}
	}

	set.addFpr16("fcw", 65, "Cwd")
	set.addFpr16("fsw", 66, "Swd")
	set.addFpr16("ftw", -1, "Ftw")
	set.addFpr16("fop", -1, "Fop")
	set.addFpr64("frip", "Rip")
	set.addFpr64("frdp", "Rdp")
	set.addFpr32("mxcsr", 64, "Mxcsr")
	set.addFpr32("mxcrmask", -1, "MxcrMask")

	for i := 0; i < 8; i++ {
		set.addFpr128("st", 33, stSpace, i) // st0, ..., st7
		set.addFpr128("mm", 41, stSpace, i) // mm0, ..., mm7
	}

	for i := 0; i < 16; i++ { // xmm0, ..., xmm15
		set.addFpr128("xmm", 17, xmmSpace, i)
	}

	for i := 0; i < 8; i++ {
		set.addDr64(i)
	}

	return set
}
