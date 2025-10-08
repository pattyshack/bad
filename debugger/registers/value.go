package registers

import (
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"
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
type Value interface {
	Size() uintptr
	IsFloat() bool

	ToBytes() []byte

	ToUint32() uint32
	ToUint64() uint64
	ToUint128() Uint128

	String() string
}

type Uint128 struct {
	High uint64
	Low  uint64
}

func (Uint128) Size() uintptr {
	return 16
}

func (Uint128) IsFloat() bool {
	return false
}

func (u Uint128) ToBytes() []byte {
	bytes := make([]byte, 16)

	_, err := binary.Encode(bytes[:8], binary.LittleEndian, u.Low)
	if err != nil {
		panic(err)
	}

	_, err = binary.Encode(bytes[8:], binary.LittleEndian, u.High)
	if err != nil {
		panic(err)
	}

	return bytes
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

func (u Uint128) String() string {
	return fmt.Sprintf("0x%016x:0x%016x", u.High, u.Low)
}

func U128(high uint64, low uint64) Uint128 {
	return Uint128{
		High: high,
		Low:  low,
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

func (u Uint[T]) ToBytes() []byte {
	bytes := make([]byte, u.Size())

	_, err := binary.Encode(bytes, binary.LittleEndian, u.Value)
	if err != nil {
		panic(err)
	}

	return bytes
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

func (u Uint[T]) String() string {
	return fmt.Sprintf(fmt.Sprintf("0x%%0%dx", u.Size()*2), u.Value)
}

type Uint8 = Uint[uint8]

func U8(v uint8) Value {
	return Uint8{
		Value: v,
	}
}

type Uint16 = Uint[uint16]

func U16(v uint16) Value {
	return Uint16{
		Value: v,
	}
}

type Uint32 = Uint[uint32]

func U32(v uint32) Value {
	return Uint32{
		Value: v,
	}
}

type Uint64 = Uint[uint64]

func U64(v uint64) Value {
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

func (i Int[T]) ToBytes() []byte {
	bytes := make([]byte, i.Size())

	_, err := binary.Encode(bytes, binary.LittleEndian, i.Value)
	if err != nil {
		panic(err)
	}

	return bytes
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
	return U128(high, low)
}

func (u Int[T]) String() string {
	return fmt.Sprintf(fmt.Sprintf("0x%%0%dx", u.Size()*2), u.Value)
}

type Int8 = Int[int8]

func I8(v int8) Value {
	return Int[int8]{
		Value: v,
	}
}

type Int16 = Int[int16]

func I16(v int16) Value {
	return Int[int16]{
		Value: v,
	}
}

type Int32 = Int[int32]

func I32(v int32) Value {
	return Int[int32]{
		Value: v,
	}
}

type Int64 = Int[int64]

func I64(v int64) Value {
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

func (f Float32) ToBytes() []byte {
	bytes := make([]byte, 4)

	_, err := binary.Encode(bytes, binary.LittleEndian, f)
	if err != nil {
		panic(err)
	}

	return bytes
}

func (f Float32) ToUint32() uint32 {
	return math.Float32bits(float32(f))
}

func (f Float32) ToUint64() uint64 {
	return uint64(f.ToUint32())
}

func (f Float32) ToUint128() Uint128 {
	return U128(0, f.ToUint64())
}

func (f Float32) String() string {
	return fmt.Sprintf("f:%f", f)
}

func F32(v float32) Value {
	return Float32(v)
}

type Float64 float64

func (Float64) Size() uintptr {
	return 8
}

func (Float64) IsFloat() bool {
	return true
}

func (f Float64) ToBytes() []byte {
	bytes := make([]byte, 8)

	_, err := binary.Encode(bytes, binary.LittleEndian, f)
	if err != nil {
		panic(err)
	}

	return bytes
}

func (f Float64) ToUint32() uint32 {
	return uint32(f.ToUint64())
}

func (f Float64) ToUint64() uint64 {
	return math.Float64bits(float64(f))
}

func (f Float64) ToUint128() Uint128 {
	return U128(0, f.ToUint64())
}

func (f Float64) String() string {
	return fmt.Sprintf("d:%f", f)
}

func F64(v float64) Value {
	return Float64(v)
}
