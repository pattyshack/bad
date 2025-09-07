package bad

import (
	"fmt"
	"math"
	"testing"

	"github.com/pattyshack/gt/testing/expect"
	"github.com/pattyshack/gt/testing/suite"
)

type RegistersSuite struct{}

func TestRegisters(t *testing.T) {
	suite.RunTests(t, &RegistersSuite{})
}

func (RegistersSuite) TestRax(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 0, registers["rax"].DwarfId)

	state := RegisterState{}
	state.gpr.Rax = 0x0102030405060708

	val := state.Value(registers["rax"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["eax"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["ax"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["al"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	val = state.Value(registers["ah"])
	u8, ok = val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x07, u8.Value)

	newState, err := state.WithValue(
		registers["rax"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rax)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rax)

	newState, err = state.WithValue(
		registers["eax"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rax)
	expect.Equal(t, 0x50607080, newState.gpr.Rax)

	newState, err = state.WithValue(
		registers["ax"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rax)
	expect.Equal(t, 0x7080, newState.gpr.Rax)

	newState, err = state.WithValue(
		registers["al"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rax)
	expect.Equal(t, 0x80, newState.gpr.Rax)

	newState, err = state.WithValue(
		registers["ah"],
		Uint8Value(0x70))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rax)
	expect.Equal(t, 0x7000, newState.gpr.Rax)
}

func (RegistersSuite) TestRbx(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 3, registers["rbx"].DwarfId)

	state := RegisterState{}
	state.gpr.Rbx = 0x0102030405060708

	vbl := state.Value(registers["rbx"])
	u64, ok := vbl.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	vbl = state.Value(registers["ebx"])
	u32, ok := vbl.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	vbl = state.Value(registers["bx"])
	u16, ok := vbl.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	vbl = state.Value(registers["bl"])
	u8, ok := vbl.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	vbl = state.Value(registers["bh"])
	u8, ok = vbl.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x07, u8.Value)

	bytes := uint64(0xf0e0d0c0b0a09080)
	newState, err := state.WithValue(
		registers["rbx"],
		Int64Value(int64(bytes)))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0xf0e0d0c0b0a09080, newState.gpr.Rbx)

	newState, err = state.WithValue(
		registers["rbx"],
		Int64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rbx)

	bytes = 0xf0e0d0c0
	newState, err = state.WithValue(
		registers["ebx"],
		Int32Value(int32(bytes)))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0xfffffffff0e0d0c0, newState.gpr.Rbx)

	newState, err = state.WithValue(
		registers["ebx"],
		Int32Value(0x10203040))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0x10203040, newState.gpr.Rbx)

	bytes = 0xf0e0
	newState, err = state.WithValue(
		registers["bx"],
		Int16Value(int16(bytes)))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0xfffffffffffff0e0, newState.gpr.Rbx)

	newState, err = state.WithValue(
		registers["bx"],
		Int16Value(0x1020))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0x1020, newState.gpr.Rbx)

	bytes = 0xf0
	newState, err = state.WithValue(
		registers["bl"],
		Int8Value(int8(bytes)))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0xfffffffffffffff0, newState.gpr.Rbx)

	newState, err = state.WithValue(
		registers["bl"],
		Int8Value(0x10))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0x10, newState.gpr.Rbx)

	bytes = 0xf1
	newState, err = state.WithValue(
		registers["bh"],
		Int8Value(int8(bytes)))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0xfffffffffffff100, newState.gpr.Rbx)

	newState, err = state.WithValue(
		registers["bh"],
		Int8Value(0x12))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0x1200, newState.gpr.Rbx)
}

func (RegistersSuite) TestRcx(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 2, registers["rcx"].DwarfId)

	state := RegisterState{}
	state.gpr.Rcx = 0x0102030405060708

	val := state.Value(registers["rcx"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["ecx"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["cx"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["cl"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	val = state.Value(registers["ch"])
	u8, ok = val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x07, u8.Value)

	newState, err := state.WithValue(
		registers["rcx"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rcx)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rcx)

	newState, err = state.WithValue(
		registers["ecx"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rcx)
	expect.Equal(t, 0x50607080, newState.gpr.Rcx)

	newState, err = state.WithValue(
		registers["cx"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rcx)
	expect.Equal(t, 0x7080, newState.gpr.Rcx)

	newState, err = state.WithValue(
		registers["cl"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rcx)
	expect.Equal(t, 0x80, newState.gpr.Rcx)

	newState, err = state.WithValue(
		registers["ch"],
		Uint8Value(0x70))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rcx)
	expect.Equal(t, 0x7000, newState.gpr.Rcx)
}

func (RegistersSuite) TestRdx(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 1, registers["rdx"].DwarfId)

	state := RegisterState{}
	state.gpr.Rdx = 0x0102030405060708

	val := state.Value(registers["rdx"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["edx"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["dx"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["dl"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	val = state.Value(registers["dh"])
	u8, ok = val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x07, u8.Value)

	newState, err := state.WithValue(
		registers["rdx"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdx)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rdx)

	newState, err = state.WithValue(
		registers["edx"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdx)
	expect.Equal(t, 0x50607080, newState.gpr.Rdx)

	newState, err = state.WithValue(
		registers["dx"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdx)
	expect.Equal(t, 0x7080, newState.gpr.Rdx)

	newState, err = state.WithValue(
		registers["dl"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdx)
	expect.Equal(t, 0x80, newState.gpr.Rdx)

	newState, err = state.WithValue(
		registers["dh"],
		Uint8Value(0x70))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdx)
	expect.Equal(t, 0x7000, newState.gpr.Rdx)
}

func (RegistersSuite) TestRsi(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 4, registers["rsi"].DwarfId)

	state := RegisterState{}
	state.gpr.Rsi = 0x0102030405060708

	val := state.Value(registers["rsi"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["esi"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["si"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["sil"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["rsi"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsi)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rsi)

	newState, err = state.WithValue(
		registers["esi"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsi)
	expect.Equal(t, 0x50607080, newState.gpr.Rsi)

	newState, err = state.WithValue(
		registers["si"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsi)
	expect.Equal(t, 0x7080, newState.gpr.Rsi)

	newState, err = state.WithValue(
		registers["sil"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsi)
	expect.Equal(t, 0x80, newState.gpr.Rsi)
}

func (RegistersSuite) TestRdi(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 5, registers["rdi"].DwarfId)

	state := RegisterState{}
	state.gpr.Rdi = 0x0102030405060708

	val := state.Value(registers["rdi"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["edi"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["di"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["dil"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["rdi"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdi)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rdi)

	newState, err = state.WithValue(
		registers["edi"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdi)
	expect.Equal(t, 0x50607080, newState.gpr.Rdi)

	newState, err = state.WithValue(
		registers["di"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdi)
	expect.Equal(t, 0x7080, newState.gpr.Rdi)

	newState, err = state.WithValue(
		registers["dil"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdi)
	expect.Equal(t, 0x80, newState.gpr.Rdi)
}

func (RegistersSuite) TestRbp(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 6, registers["rbp"].DwarfId)

	state := RegisterState{}
	state.gpr.Rbp = 0x0102030405060708

	val := state.Value(registers["rbp"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["ebp"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["bp"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["bpl"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["rbp"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbp)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rbp)

	newState, err = state.WithValue(
		registers["ebp"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbp)
	expect.Equal(t, 0x50607080, newState.gpr.Rbp)

	newState, err = state.WithValue(
		registers["bp"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbp)
	expect.Equal(t, 0x7080, newState.gpr.Rbp)

	newState, err = state.WithValue(
		registers["bpl"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbp)
	expect.Equal(t, 0x80, newState.gpr.Rbp)
}

func (RegistersSuite) TestRsp(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 7, registers["rsp"].DwarfId)

	state := RegisterState{}
	state.gpr.Rsp = 0x0102030405060708

	val := state.Value(registers["rsp"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["esp"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["sp"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["spl"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["rsp"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsp)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rsp)

	newState, err = state.WithValue(
		registers["esp"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsp)
	expect.Equal(t, 0x50607080, newState.gpr.Rsp)

	newState, err = state.WithValue(
		registers["sp"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsp)
	expect.Equal(t, 0x7080, newState.gpr.Rsp)

	newState, err = state.WithValue(
		registers["spl"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsp)
	expect.Equal(t, 0x80, newState.gpr.Rsp)
}

func (RegistersSuite) TestR8(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 8, registers["r8"].DwarfId)

	state := RegisterState{}
	state.gpr.R8 = 0x0102030405060708

	val := state.Value(registers["r8"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["r8d"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["r8w"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["r8b"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["r8"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R8)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R8)

	newState, err = state.WithValue(
		registers["r8d"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R8)
	expect.Equal(t, 0x50607080, newState.gpr.R8)

	newState, err = state.WithValue(
		registers["r8w"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R8)
	expect.Equal(t, 0x7080, newState.gpr.R8)

	newState, err = state.WithValue(
		registers["r8b"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R8)
	expect.Equal(t, 0x80, newState.gpr.R8)
}

func (RegistersSuite) TestR9(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 9, registers["r9"].DwarfId)

	state := RegisterState{}
	state.gpr.R9 = 0x0102030405060708

	val := state.Value(registers["r9"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["r9d"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["r9w"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["r9b"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["r9"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R9)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R9)

	newState, err = state.WithValue(
		registers["r9d"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R9)
	expect.Equal(t, 0x50607080, newState.gpr.R9)

	newState, err = state.WithValue(
		registers["r9w"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R9)
	expect.Equal(t, 0x7080, newState.gpr.R9)

	newState, err = state.WithValue(
		registers["r9b"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R9)
	expect.Equal(t, 0x80, newState.gpr.R9)
}

func (RegistersSuite) TestR10(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 10, registers["r10"].DwarfId)

	state := RegisterState{}
	state.gpr.R10 = 0x0102030405060708

	val := state.Value(registers["r10"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["r10d"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["r10w"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["r10b"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["r10"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R10)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R10)

	newState, err = state.WithValue(
		registers["r10d"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R10)
	expect.Equal(t, 0x50607080, newState.gpr.R10)

	newState, err = state.WithValue(
		registers["r10w"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R10)
	expect.Equal(t, 0x7080, newState.gpr.R10)

	newState, err = state.WithValue(
		registers["r10b"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R10)
	expect.Equal(t, 0x80, newState.gpr.R10)
}

func (RegistersSuite) TestR11(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 11, registers["r11"].DwarfId)

	state := RegisterState{}
	state.gpr.R11 = 0x0102030405060708

	val := state.Value(registers["r11"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["r11d"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["r11w"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["r11b"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["r11"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R11)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R11)

	newState, err = state.WithValue(
		registers["r11d"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R11)
	expect.Equal(t, 0x50607080, newState.gpr.R11)

	newState, err = state.WithValue(
		registers["r11w"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R11)
	expect.Equal(t, 0x7080, newState.gpr.R11)

	newState, err = state.WithValue(
		registers["r11b"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R11)
	expect.Equal(t, 0x80, newState.gpr.R11)
}

func (RegistersSuite) TestR12(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 12, registers["r12"].DwarfId)

	state := RegisterState{}
	state.gpr.R12 = 0x0102030405060708

	val := state.Value(registers["r12"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["r12d"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["r12w"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["r12b"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["r12"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R12)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R12)

	newState, err = state.WithValue(
		registers["r12d"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R12)
	expect.Equal(t, 0x50607080, newState.gpr.R12)

	newState, err = state.WithValue(
		registers["r12w"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R12)
	expect.Equal(t, 0x7080, newState.gpr.R12)

	newState, err = state.WithValue(
		registers["r12b"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R12)
	expect.Equal(t, 0x80, newState.gpr.R12)
}

func (RegistersSuite) TestR13(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 13, registers["r13"].DwarfId)

	state := RegisterState{}
	state.gpr.R13 = 0x0102030405060708

	val := state.Value(registers["r13"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["r13d"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["r13w"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["r13b"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["r13"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R13)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R13)

	newState, err = state.WithValue(
		registers["r13d"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R13)
	expect.Equal(t, 0x50607080, newState.gpr.R13)

	newState, err = state.WithValue(
		registers["r13w"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R13)
	expect.Equal(t, 0x7080, newState.gpr.R13)

	newState, err = state.WithValue(
		registers["r13b"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R13)
	expect.Equal(t, 0x80, newState.gpr.R13)
}

func (RegistersSuite) TestR14(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 14, registers["r14"].DwarfId)

	state := RegisterState{}
	state.gpr.R14 = 0x0102030405060708

	val := state.Value(registers["r14"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["r14d"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["r14w"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["r14b"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["r14"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R14)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R14)

	newState, err = state.WithValue(
		registers["r14d"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R14)
	expect.Equal(t, 0x50607080, newState.gpr.R14)

	newState, err = state.WithValue(
		registers["r14w"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R14)
	expect.Equal(t, 0x7080, newState.gpr.R14)

	newState, err = state.WithValue(
		registers["r14b"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R14)
	expect.Equal(t, 0x80, newState.gpr.R14)
}

func (RegistersSuite) TestR15(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 15, registers["r15"].DwarfId)

	state := RegisterState{}
	state.gpr.R15 = 0x0102030405060708

	val := state.Value(registers["r15"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(registers["r15d"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(registers["r15w"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(registers["r15b"])
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		registers["r15"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R15)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R15)

	newState, err = state.WithValue(
		registers["r15d"],
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R15)
	expect.Equal(t, 0x50607080, newState.gpr.R15)

	newState, err = state.WithValue(
		registers["r15w"],
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R15)
	expect.Equal(t, 0x7080, newState.gpr.R15)

	newState, err = state.WithValue(
		registers["r15b"],
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R15)
	expect.Equal(t, 0x80, newState.gpr.R15)
}

func (RegistersSuite) TestRip(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 16, registers["rip"].DwarfId)

	state := RegisterState{}
	state.gpr.Rip = 0x0102030405060708

	val := state.Value(registers["rip"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		registers["rip"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rip)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rip)
}

func (RegistersSuite) TestEflags(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 49, registers["eflags"].DwarfId)

	state := RegisterState{}
	state.gpr.Eflags = 0x0102030405060708

	val := state.Value(registers["eflags"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		registers["eflags"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Eflags)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Eflags)
}

func (RegistersSuite) TestCs(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 51, registers["cs"].DwarfId)

	state := RegisterState{}
	state.gpr.Cs = 0x0102030405060708

	val := state.Value(registers["cs"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		registers["cs"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Cs)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Cs)
}

func (RegistersSuite) TestFs(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 54, registers["fs"].DwarfId)

	state := RegisterState{}
	state.gpr.Fs = 0x0102030405060708

	val := state.Value(registers["fs"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		registers["fs"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Fs)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Fs)
}

func (RegistersSuite) TestGs(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 55, registers["gs"].DwarfId)

	state := RegisterState{}
	state.gpr.Gs = 0x0102030405060708

	val := state.Value(registers["gs"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		registers["gs"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Gs)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Gs)
}

func (RegistersSuite) TestSs(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 52, registers["ss"].DwarfId)

	state := RegisterState{}
	state.gpr.Ss = 0x0102030405060708

	val := state.Value(registers["ss"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		registers["ss"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Ss)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Ss)
}

func (RegistersSuite) TestDs(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 53, registers["ds"].DwarfId)

	state := RegisterState{}
	state.gpr.Ds = 0x0102030405060708

	val := state.Value(registers["ds"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		registers["ds"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Ds)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Ds)
}

func (RegistersSuite) TestEs(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 50, registers["es"].DwarfId)

	state := RegisterState{}
	state.gpr.Es = 0x0102030405060708

	val := state.Value(registers["es"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		registers["es"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Es)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Es)
}

func (RegistersSuite) TestFcw(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 65, registers["fcw"].DwarfId)

	state := RegisterState{}
	state.fpr.Cwd = 0x0102

	val := state.Value(registers["fcw"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0102, u16.Value)

	newState, err := state.WithValue(registers["fcw"], Uint16Value(0x1020))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102, state.fpr.Cwd)
	expect.Equal(t, 0x1020, newState.fpr.Cwd)
}

func (RegistersSuite) TestFsw(t *testing.T) {
	registers := NewRegisterSet()
	expect.Equal(t, 66, registers["fsw"].DwarfId)

	state := RegisterState{}
	state.fpr.Swd = 0x0102

	val := state.Value(registers["fsw"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0102, u16.Value)

	newState, err := state.WithValue(registers["fsw"], Uint16Value(0x1020))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102, state.fpr.Swd)
	expect.Equal(t, 0x1020, newState.fpr.Swd)
}

func (RegistersSuite) TestFtw(t *testing.T) {
	registers := NewRegisterSet()

	state := RegisterState{}
	state.fpr.Ftw = 0x0102

	val := state.Value(registers["ftw"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0102, u16.Value)

	newState, err := state.WithValue(registers["ftw"], Uint16Value(0x1020))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102, state.fpr.Ftw)
	expect.Equal(t, 0x1020, newState.fpr.Ftw)
}

func (RegistersSuite) TestFop(t *testing.T) {
	registers := NewRegisterSet()

	state := RegisterState{}
	state.fpr.Fop = 0x0102

	val := state.Value(registers["fop"])
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0102, u16.Value)

	newState, err := state.WithValue(registers["fop"], Uint16Value(0x1020))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102, state.fpr.Fop)
	expect.Equal(t, 0x1020, newState.fpr.Fop)
}

func (RegistersSuite) TestFrip(t *testing.T) {
	registers := NewRegisterSet()

	state := RegisterState{}
	state.fpr.Rip = 0x0102030405060708

	val := state.Value(registers["frip"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		registers["frip"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.fpr.Rip)
	expect.Equal(t, 0x1020304050607080, newState.fpr.Rip)
}

func (RegistersSuite) TestFrdp(t *testing.T) {
	registers := NewRegisterSet()

	state := RegisterState{}
	state.fpr.Rdp = 0x0102030405060708

	val := state.Value(registers["frdp"])
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		registers["frdp"],
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.fpr.Rdp)
	expect.Equal(t, 0x1020304050607080, newState.fpr.Rdp)
}

func (RegistersSuite) TestMxcsr(t *testing.T) {
	registers := NewRegisterSet()

	state := RegisterState{}
	state.fpr.Mxcsr = 0x01020304

	val := state.Value(registers["mxcsr"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x01020304, u32.Value)

	newState, err := state.WithValue(
		registers["mxcsr"],
		Uint32Value(0x10203040))
	expect.Nil(t, err)
	expect.Equal(t, 0x01020304, state.fpr.Mxcsr)
	expect.Equal(t, 0x10203040, newState.fpr.Mxcsr)
}

func (RegistersSuite) TestMxcrMask(t *testing.T) {
	registers := NewRegisterSet()

	state := RegisterState{}
	state.fpr.MxcrMask = 0x01020304

	val := state.Value(registers["mxcrmask"])
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x01020304, u32.Value)

	newState, err := state.WithValue(
		registers["mxcrmask"],
		Uint32Value(0x10203040))
	expect.Nil(t, err)
	expect.Equal(t, 0x01020304, state.fpr.MxcrMask)
	expect.Equal(t, 0x10203040, newState.fpr.MxcrMask)
}

func TestStMm(t *testing.T) {
	registers := NewRegisterSet()

	for i := 0; i < 8; i++ {
		st := fmt.Sprintf("st%d", i)
		mm := fmt.Sprintf("mm%d", i)

		state := RegisterState{}

		lowIdx := 2 * i
		highIdx := 2*i + 1

		low := uint64((i + 1) * 100)
		high := ^low

		state.fpr.StSpace[lowIdx] = low
		state.fpr.StSpace[highIdx] = high

		val := state.Value(registers[st])
		u128, ok := val.(Uint128)
		expect.True(t, ok)
		expect.Equal(t, low, u128.Low)
		expect.Equal(t, high, u128.High)

		val = state.Value(registers[mm])
		u128, ok = val.(Uint128)
		expect.True(t, ok)
		expect.Equal(t, low, u128.Low)
		expect.Equal(t, high, u128.High)

		newLow := low + 1
		newHigh := ^newLow

		newState, err := state.WithValue(
			registers[st],
			Uint128Value(newLow, newHigh))
		expect.Nil(t, err)
		expect.Equal(t, low, state.fpr.StSpace[lowIdx])
		expect.Equal(t, high, state.fpr.StSpace[highIdx])
		expect.Equal(t, newLow, newState.fpr.StSpace[lowIdx])
		expect.Equal(t, newHigh, newState.fpr.StSpace[highIdx])

		newLow += 1
		newHigh = ^newHigh

		newState, err = state.WithValue(
			registers[mm],
			Uint128Value(newLow, newHigh))
		expect.Nil(t, err)
		expect.Equal(t, low, state.fpr.StSpace[lowIdx])
		expect.Equal(t, high, state.fpr.StSpace[highIdx])
		expect.Equal(t, newLow, newState.fpr.StSpace[lowIdx])
		expect.Equal(t, newHigh, newState.fpr.StSpace[highIdx])

		state.fpr.StSpace[lowIdx] = 0xabc
		state.fpr.StSpace[highIdx] = 0xdef

		newLow = 0x0102030405060708
		newState, err = state.WithValue(
			registers[mm],
			Float64Value(math.Float64frombits(newLow)))
		expect.Nil(t, err)
		expect.Equal(t, 0xabc, state.fpr.StSpace[lowIdx])
		expect.Equal(t, 0xdef, state.fpr.StSpace[highIdx])
		expect.Equal(t, newLow, newState.fpr.StSpace[lowIdx])
		expect.Equal(t, 0, newState.fpr.StSpace[highIdx])

		newLow = 0x01020304
		newState, err = state.WithValue(
			registers[st],
			Float32Value(math.Float32frombits(uint32(newLow))))
		expect.Nil(t, err)
		expect.Equal(t, 0xabc, state.fpr.StSpace[lowIdx])
		expect.Equal(t, 0xdef, state.fpr.StSpace[highIdx])
		expect.Equal(t, newLow, newState.fpr.StSpace[lowIdx])
		expect.Equal(t, 0, newState.fpr.StSpace[highIdx])
	}

}

func TestXmm(t *testing.T) {
	registers := NewRegisterSet()

	for i := 0; i < 16; i++ {
		xmm := fmt.Sprintf("xmm%d", i)

		state := RegisterState{}

		lowIdx := 2 * i
		highIdx := 2*i + 1

		low := uint64((i + 1) * 100)
		high := ^low

		state.fpr.XmmSpace[lowIdx] = low
		state.fpr.XmmSpace[highIdx] = high

		val := state.Value(registers[xmm])
		u128, ok := val.(Uint128)
		expect.True(t, ok)
		expect.Equal(t, low, u128.Low)
		expect.Equal(t, high, u128.High)

		newLow := low + 1
		newHigh := ^newLow

		newState, err := state.WithValue(
			registers[xmm],
			Uint128Value(newLow, newHigh))
		expect.Nil(t, err)
		expect.Equal(t, low, state.fpr.XmmSpace[lowIdx])
		expect.Equal(t, newLow, newState.fpr.XmmSpace[lowIdx])
		expect.Equal(t, high, state.fpr.XmmSpace[highIdx])
		expect.Equal(t, newHigh, newState.fpr.XmmSpace[highIdx])

		state.fpr.XmmSpace[lowIdx] = 0xabc
		state.fpr.XmmSpace[highIdx] = 0xdef

		newLow = 0x0102030405060708
		newState, err = state.WithValue(
			registers[xmm],
			Float64Value(math.Float64frombits(newLow)))
		expect.Nil(t, err)
		expect.Equal(t, 0xabc, state.fpr.XmmSpace[lowIdx])
		expect.Equal(t, 0xdef, state.fpr.XmmSpace[highIdx])
		expect.Equal(t, newLow, newState.fpr.XmmSpace[lowIdx])
		expect.Equal(t, 0, newState.fpr.XmmSpace[highIdx])

		newLow = 0x01020304
		newState, err = state.WithValue(
			registers[xmm],
			Float32Value(math.Float32frombits(uint32(newLow))))
		expect.Nil(t, err)
		expect.Equal(t, 0xabc, state.fpr.XmmSpace[lowIdx])
		expect.Equal(t, 0xdef, state.fpr.XmmSpace[highIdx])
		expect.Equal(t, newLow, newState.fpr.XmmSpace[lowIdx])
		expect.Equal(t, 0, newState.fpr.XmmSpace[highIdx])
	}
}

func (RegistersSuite) TestDr(t *testing.T) {
	registers := NewRegisterSet()

	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("dr%d", i)

		value := uint64((i + 1) * 10)

		state := RegisterState{}
		state.dr[i] = uintptr(value)

		val := state.Value(registers[name])
		u64, ok := val.(Uint64)
		expect.True(t, ok)
		expect.Equal(t, value, u64.Value)

		newState, err := state.WithValue(registers[name], Uint64Value(value+1))
		if i == 4 || i == 5 {
			expect.Error(t, err, "read-only")
		} else {
			expect.Nil(t, err)
			expect.Equal(t, uintptr(value), state.dr[i])
			expect.Equal(t, uintptr(value+1), newState.dr[i])
		}
	}
}
