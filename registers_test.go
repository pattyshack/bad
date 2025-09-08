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

	rax, ok := registers.RegisterByName("rax")
	expect.True(t, ok)
	expect.Equal(t, 0, rax.DwarfId)

	eax, ok := registers.RegisterByName("eax")
	expect.True(t, ok)

	ax, ok := registers.RegisterByName("ax")
	expect.True(t, ok)

	ah, ok := registers.RegisterByName("ah")
	expect.True(t, ok)

	al, ok := registers.RegisterByName("al")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.Rax = 0x0102030405060708

	val := state.Value(rax)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(eax)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(ax)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(al)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	val = state.Value(ah)
	u8, ok = val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x07, u8.Value)

	newState, err := state.WithValue(rax, Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rax)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rax)

	newState, err = state.WithValue(eax, Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rax)
	expect.Equal(t, 0x50607080, newState.gpr.Rax)

	newState, err = state.WithValue(ax, Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rax)
	expect.Equal(t, 0x7080, newState.gpr.Rax)

	newState, err = state.WithValue(al, Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rax)
	expect.Equal(t, 0x80, newState.gpr.Rax)

	newState, err = state.WithValue(ah, Uint8Value(0x70))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rax)
	expect.Equal(t, 0x7000, newState.gpr.Rax)
}

func (RegistersSuite) TestRbx(t *testing.T) {
	registers := NewRegisterSet()

	rbx, ok := registers.RegisterByName("rbx")
	expect.True(t, ok)
	expect.Equal(t, 3, rbx.DwarfId)

	ebx, ok := registers.RegisterByName("ebx")
	expect.True(t, ok)

	bx, ok := registers.RegisterByName("bx")
	expect.True(t, ok)

	bh, ok := registers.RegisterByName("bh")
	expect.True(t, ok)

	bl, ok := registers.RegisterByName("bl")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.Rbx = 0x0102030405060708

	vbl := state.Value(rbx)
	u64, ok := vbl.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	vbl = state.Value(ebx)
	u32, ok := vbl.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	vbl = state.Value(bx)
	u16, ok := vbl.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	vbl = state.Value(bl)
	u8, ok := vbl.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	vbl = state.Value(bh)
	u8, ok = vbl.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x07, u8.Value)

	bytes := uint64(0xf0e0d0c0b0a09080)
	newState, err := state.WithValue(rbx, Int64Value(int64(bytes)))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0xf0e0d0c0b0a09080, newState.gpr.Rbx)

	newState, err = state.WithValue(rbx, Int64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rbx)

	bytes = 0xf0e0d0c0
	newState, err = state.WithValue(ebx, Int32Value(int32(bytes)))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0xfffffffff0e0d0c0, newState.gpr.Rbx)

	newState, err = state.WithValue(ebx, Int32Value(0x10203040))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0x10203040, newState.gpr.Rbx)

	bytes = 0xf0e0
	newState, err = state.WithValue(bx, Int16Value(int16(bytes)))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0xfffffffffffff0e0, newState.gpr.Rbx)

	newState, err = state.WithValue(bx, Int16Value(0x1020))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0x1020, newState.gpr.Rbx)

	bytes = 0xf0
	newState, err = state.WithValue(bl, Int8Value(int8(bytes)))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0xfffffffffffffff0, newState.gpr.Rbx)

	newState, err = state.WithValue(bl, Int8Value(0x10))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0x10, newState.gpr.Rbx)

	bytes = 0xf1
	newState, err = state.WithValue(bh, Int8Value(int8(bytes)))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0xfffffffffffff100, newState.gpr.Rbx)

	newState, err = state.WithValue(bh, Int8Value(0x12))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbx)
	expect.Equal(t, 0x1200, newState.gpr.Rbx)
}

func (RegistersSuite) TestRcx(t *testing.T) {
	registers := NewRegisterSet()

	rcx, ok := registers.RegisterByName("rcx")
	expect.True(t, ok)
	expect.Equal(t, 2, rcx.DwarfId)

	ecx, ok := registers.RegisterByName("ecx")
	expect.True(t, ok)

	cx, ok := registers.RegisterByName("cx")
	expect.True(t, ok)

	ch, ok := registers.RegisterByName("ch")
	expect.True(t, ok)

	cl, ok := registers.RegisterByName("cl")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.Rcx = 0x0102030405060708

	val := state.Value(rcx)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(ecx)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(cx)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(cl)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	val = state.Value(ch)
	u8, ok = val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x07, u8.Value)

	newState, err := state.WithValue(rcx, Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rcx)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rcx)

	newState, err = state.WithValue(ecx, Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rcx)
	expect.Equal(t, 0x50607080, newState.gpr.Rcx)

	newState, err = state.WithValue(cx, Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rcx)
	expect.Equal(t, 0x7080, newState.gpr.Rcx)

	newState, err = state.WithValue(cl, Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rcx)
	expect.Equal(t, 0x80, newState.gpr.Rcx)

	newState, err = state.WithValue(ch, Uint8Value(0x70))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rcx)
	expect.Equal(t, 0x7000, newState.gpr.Rcx)
}

func (RegistersSuite) TestRdx(t *testing.T) {
	registers := NewRegisterSet()

	rdx, ok := registers.RegisterByName("rdx")
	expect.True(t, ok)
	expect.Equal(t, 1, rdx.DwarfId)

	edx, ok := registers.RegisterByName("edx")
	expect.True(t, ok)

	dx, ok := registers.RegisterByName("dx")
	expect.True(t, ok)

	dh, ok := registers.RegisterByName("dh")
	expect.True(t, ok)

	dl, ok := registers.RegisterByName("dl")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.Rdx = 0x0102030405060708

	val := state.Value(rdx)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(edx)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(dx)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(dl)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	val = state.Value(dh)
	u8, ok = val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x07, u8.Value)

	newState, err := state.WithValue(rdx, Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdx)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rdx)

	newState, err = state.WithValue(edx, Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdx)
	expect.Equal(t, 0x50607080, newState.gpr.Rdx)

	newState, err = state.WithValue(dx, Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdx)
	expect.Equal(t, 0x7080, newState.gpr.Rdx)

	newState, err = state.WithValue(dl, Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdx)
	expect.Equal(t, 0x80, newState.gpr.Rdx)

	newState, err = state.WithValue(dh, Uint8Value(0x70))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdx)
	expect.Equal(t, 0x7000, newState.gpr.Rdx)
}

func (RegistersSuite) TestRsi(t *testing.T) {
	registers := NewRegisterSet()

	rsi, ok := registers.RegisterByName("rsi")
	expect.True(t, ok)
	expect.Equal(t, 4, rsi.DwarfId)

	esi, ok := registers.RegisterByName("esi")
	expect.True(t, ok)

	si, ok := registers.RegisterByName("si")
	expect.True(t, ok)

	sil, ok := registers.RegisterByName("sil")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.Rsi = 0x0102030405060708

	val := state.Value(rsi)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(esi)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(si)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(sil)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(rsi, Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsi)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rsi)

	newState, err = state.WithValue(esi, Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsi)
	expect.Equal(t, 0x50607080, newState.gpr.Rsi)

	newState, err = state.WithValue(si, Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsi)
	expect.Equal(t, 0x7080, newState.gpr.Rsi)

	newState, err = state.WithValue(sil, Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsi)
	expect.Equal(t, 0x80, newState.gpr.Rsi)
}

func (RegistersSuite) TestRdi(t *testing.T) {
	registers := NewRegisterSet()

	rdi, ok := registers.RegisterByName("rdi")
	expect.True(t, ok)
	expect.Equal(t, 5, rdi.DwarfId)

	edi, ok := registers.RegisterByName("edi")
	expect.True(t, ok)

	di, ok := registers.RegisterByName("di")
	expect.True(t, ok)

	dil, ok := registers.RegisterByName("dil")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.Rdi = 0x0102030405060708

	val := state.Value(rdi)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(edi)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(di)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(dil)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(rdi, Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdi)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rdi)

	newState, err = state.WithValue(edi, Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdi)
	expect.Equal(t, 0x50607080, newState.gpr.Rdi)

	newState, err = state.WithValue(di, Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdi)
	expect.Equal(t, 0x7080, newState.gpr.Rdi)

	newState, err = state.WithValue(dil, Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rdi)
	expect.Equal(t, 0x80, newState.gpr.Rdi)
}

func (RegistersSuite) TestRbp(t *testing.T) {
	registers := NewRegisterSet()

	rbp, ok := registers.RegisterByName("rbp")
	expect.True(t, ok)
	expect.Equal(t, 6, rbp.DwarfId)

	ebp, ok := registers.RegisterByName("ebp")
	expect.True(t, ok)

	bp, ok := registers.RegisterByName("bp")
	expect.True(t, ok)

	bpl, ok := registers.RegisterByName("bpl")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.Rbp = 0x0102030405060708

	val := state.Value(rbp)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(ebp)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(bp)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(bpl)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(rbp, Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbp)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rbp)

	newState, err = state.WithValue(ebp, Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbp)
	expect.Equal(t, 0x50607080, newState.gpr.Rbp)

	newState, err = state.WithValue(bp, Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbp)
	expect.Equal(t, 0x7080, newState.gpr.Rbp)

	newState, err = state.WithValue(bpl, Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rbp)
	expect.Equal(t, 0x80, newState.gpr.Rbp)
}

func (RegistersSuite) TestRsp(t *testing.T) {
	registers := NewRegisterSet()

	rsp, ok := registers.RegisterByName("rsp")
	expect.True(t, ok)
	expect.Equal(t, 7, rsp.DwarfId)

	esp, ok := registers.RegisterByName("esp")
	expect.True(t, ok)

	sp, ok := registers.RegisterByName("sp")
	expect.True(t, ok)

	spl, ok := registers.RegisterByName("spl")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.Rsp = 0x0102030405060708

	val := state.Value(rsp)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(esp)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(sp)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(spl)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(rsp, Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsp)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rsp)

	newState, err = state.WithValue(esp, Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsp)
	expect.Equal(t, 0x50607080, newState.gpr.Rsp)

	newState, err = state.WithValue(sp, Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsp)
	expect.Equal(t, 0x7080, newState.gpr.Rsp)

	newState, err = state.WithValue(spl, Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rsp)
	expect.Equal(t, 0x80, newState.gpr.Rsp)
}

func (RegistersSuite) TestR8(t *testing.T) {
	registers := NewRegisterSet()

	r8, ok := registers.RegisterByName("r8")
	expect.True(t, ok)
	expect.Equal(t, 8, r8.DwarfId)

	r8d, ok := registers.RegisterByName("r8d")
	expect.True(t, ok)

	r8w, ok := registers.RegisterByName("r8w")
	expect.True(t, ok)

	r8b, ok := registers.RegisterByName("r8b")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.R8 = 0x0102030405060708

	val := state.Value(r8)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(r8d)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(r8w)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(r8b)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(r8, Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R8)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R8)

	newState, err = state.WithValue(r8d, Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R8)
	expect.Equal(t, 0x50607080, newState.gpr.R8)

	newState, err = state.WithValue(r8w, Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R8)
	expect.Equal(t, 0x7080, newState.gpr.R8)

	newState, err = state.WithValue(r8b, Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R8)
	expect.Equal(t, 0x80, newState.gpr.R8)
}

func (RegistersSuite) TestR9(t *testing.T) {
	registers := NewRegisterSet()

	r9, ok := registers.RegisterByName("r9")
	expect.True(t, ok)
	expect.Equal(t, 9, r9.DwarfId)

	r9d, ok := registers.RegisterByName("r9d")
	expect.True(t, ok)

	r9w, ok := registers.RegisterByName("r9w")
	expect.True(t, ok)

	r9b, ok := registers.RegisterByName("r9b")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.R9 = 0x0102030405060708

	val := state.Value(r9)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(r9d)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(r9w)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(r9b)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(r9, Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R9)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R9)

	newState, err = state.WithValue(r9d, Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R9)
	expect.Equal(t, 0x50607080, newState.gpr.R9)

	newState, err = state.WithValue(r9w, Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R9)
	expect.Equal(t, 0x7080, newState.gpr.R9)

	newState, err = state.WithValue(r9b, Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R9)
	expect.Equal(t, 0x80, newState.gpr.R9)
}

func (RegistersSuite) TestR10(t *testing.T) {
	registers := NewRegisterSet()

	r10, ok := registers.RegisterByName("r10")
	expect.True(t, ok)
	expect.Equal(t, 10, r10.DwarfId)

	r10d, ok := registers.RegisterByName("r10d")
	expect.True(t, ok)

	r10w, ok := registers.RegisterByName("r10w")
	expect.True(t, ok)

	r10b, ok := registers.RegisterByName("r10b")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.R10 = 0x0102030405060708

	val := state.Value(r10)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(r10d)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(r10w)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(r10b)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(r10, Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R10)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R10)

	newState, err = state.WithValue(r10d, Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R10)
	expect.Equal(t, 0x50607080, newState.gpr.R10)

	newState, err = state.WithValue(r10w, Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R10)
	expect.Equal(t, 0x7080, newState.gpr.R10)

	newState, err = state.WithValue(r10b, Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R10)
	expect.Equal(t, 0x80, newState.gpr.R10)
}

func (RegistersSuite) TestR11(t *testing.T) {
	registers := NewRegisterSet()

	r11, ok := registers.RegisterByName("r11")
	expect.True(t, ok)
	expect.Equal(t, 11, r11.DwarfId)

	r11d, ok := registers.RegisterByName("r11d")
	expect.True(t, ok)

	r11w, ok := registers.RegisterByName("r11w")
	expect.True(t, ok)

	r11b, ok := registers.RegisterByName("r11b")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.R11 = 0x0102030405060708

	val := state.Value(r11)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(r11d)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(r11w)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(r11b)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		r11,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R11)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R11)

	newState, err = state.WithValue(
		r11d,
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R11)
	expect.Equal(t, 0x50607080, newState.gpr.R11)

	newState, err = state.WithValue(
		r11w,
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R11)
	expect.Equal(t, 0x7080, newState.gpr.R11)

	newState, err = state.WithValue(
		r11b,
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R11)
	expect.Equal(t, 0x80, newState.gpr.R11)
}

func (RegistersSuite) TestR12(t *testing.T) {
	registers := NewRegisterSet()

	r12, ok := registers.RegisterByName("r12")
	expect.True(t, ok)
	expect.Equal(t, 12, r12.DwarfId)

	r12d, ok := registers.RegisterByName("r12d")
	expect.True(t, ok)

	r12w, ok := registers.RegisterByName("r12w")
	expect.True(t, ok)

	r12b, ok := registers.RegisterByName("r12b")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.R12 = 0x0102030405060708

	val := state.Value(r12)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(r12d)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(r12w)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(r12b)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		r12,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R12)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R12)

	newState, err = state.WithValue(
		r12d,
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R12)
	expect.Equal(t, 0x50607080, newState.gpr.R12)

	newState, err = state.WithValue(
		r12w,
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R12)
	expect.Equal(t, 0x7080, newState.gpr.R12)

	newState, err = state.WithValue(
		r12b,
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R12)
	expect.Equal(t, 0x80, newState.gpr.R12)
}

func (RegistersSuite) TestR13(t *testing.T) {
	registers := NewRegisterSet()

	r13, ok := registers.RegisterByName("r13")
	expect.True(t, ok)
	expect.Equal(t, 13, r13.DwarfId)

	r13d, ok := registers.RegisterByName("r13d")
	expect.True(t, ok)

	r13w, ok := registers.RegisterByName("r13w")
	expect.True(t, ok)

	r13b, ok := registers.RegisterByName("r13b")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.R13 = 0x0102030405060708

	val := state.Value(r13)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(r13d)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(r13w)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(r13b)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		r13,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R13)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R13)

	newState, err = state.WithValue(
		r13d,
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R13)
	expect.Equal(t, 0x50607080, newState.gpr.R13)

	newState, err = state.WithValue(
		r13w,
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R13)
	expect.Equal(t, 0x7080, newState.gpr.R13)

	newState, err = state.WithValue(
		r13b,
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R13)
	expect.Equal(t, 0x80, newState.gpr.R13)
}

func (RegistersSuite) TestR14(t *testing.T) {
	registers := NewRegisterSet()

	r14, ok := registers.RegisterByName("r14")
	expect.True(t, ok)
	expect.Equal(t, 14, r14.DwarfId)

	r14d, ok := registers.RegisterByName("r14d")
	expect.True(t, ok)

	r14w, ok := registers.RegisterByName("r14w")
	expect.True(t, ok)

	r14b, ok := registers.RegisterByName("r14b")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.R14 = 0x0102030405060708

	val := state.Value(r14)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(r14d)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(r14w)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(r14b)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		r14,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R14)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R14)

	newState, err = state.WithValue(
		r14d,
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R14)
	expect.Equal(t, 0x50607080, newState.gpr.R14)

	newState, err = state.WithValue(
		r14w,
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R14)
	expect.Equal(t, 0x7080, newState.gpr.R14)

	newState, err = state.WithValue(
		r14b,
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R14)
	expect.Equal(t, 0x80, newState.gpr.R14)
}

func (RegistersSuite) TestR15(t *testing.T) {
	registers := NewRegisterSet()

	r15, ok := registers.RegisterByName("r15")
	expect.True(t, ok)
	expect.Equal(t, 15, r15.DwarfId)

	r15d, ok := registers.RegisterByName("r15d")
	expect.True(t, ok)

	r15w, ok := registers.RegisterByName("r15w")
	expect.True(t, ok)

	r15b, ok := registers.RegisterByName("r15b")
	expect.True(t, ok)

	state := RegisterState{}
	state.gpr.R15 = 0x0102030405060708

	val := state.Value(r15)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	val = state.Value(r15d)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x05060708, u32.Value)

	val = state.Value(r15w)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0708, u16.Value)

	val = state.Value(r15b)
	u8, ok := val.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x08, u8.Value)

	newState, err := state.WithValue(
		r15,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R15)
	expect.Equal(t, 0x1020304050607080, newState.gpr.R15)

	newState, err = state.WithValue(
		r15d,
		Uint32Value(0x50607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R15)
	expect.Equal(t, 0x50607080, newState.gpr.R15)

	newState, err = state.WithValue(
		r15w,
		Uint16Value(0x7080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R15)
	expect.Equal(t, 0x7080, newState.gpr.R15)

	newState, err = state.WithValue(
		r15b,
		Uint8Value(0x80))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.R15)
	expect.Equal(t, 0x80, newState.gpr.R15)
}

func (RegistersSuite) TestRip(t *testing.T) {
	registers := NewRegisterSet()

	rip, ok := registers.RegisterByName("rip")
	expect.True(t, ok)
	expect.Equal(t, 16, rip.DwarfId)

	state := RegisterState{}
	state.gpr.Rip = 0x0102030405060708

	val := state.Value(rip)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		rip,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Rip)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Rip)
}

func (RegistersSuite) TestEflags(t *testing.T) {
	registers := NewRegisterSet()

	eflags, ok := registers.RegisterByName("eflags")
	expect.True(t, ok)
	expect.Equal(t, 49, eflags.DwarfId)

	state := RegisterState{}
	state.gpr.Eflags = 0x0102030405060708

	val := state.Value(eflags)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		eflags,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Eflags)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Eflags)
}

func (RegistersSuite) TestCs(t *testing.T) {
	registers := NewRegisterSet()

	cs, ok := registers.RegisterByName("cs")
	expect.True(t, ok)
	expect.Equal(t, 51, cs.DwarfId)

	state := RegisterState{}
	state.gpr.Cs = 0x0102030405060708

	val := state.Value(cs)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		cs,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Cs)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Cs)
}

func (RegistersSuite) TestFs(t *testing.T) {
	registers := NewRegisterSet()

	fs, ok := registers.RegisterByName("fs")
	expect.True(t, ok)
	expect.Equal(t, 54, fs.DwarfId)

	state := RegisterState{}
	state.gpr.Fs = 0x0102030405060708

	val := state.Value(fs)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		fs,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Fs)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Fs)
}

func (RegistersSuite) TestGs(t *testing.T) {
	registers := NewRegisterSet()

	gs, ok := registers.RegisterByName("gs")
	expect.True(t, ok)
	expect.Equal(t, 55, gs.DwarfId)

	state := RegisterState{}
	state.gpr.Gs = 0x0102030405060708

	val := state.Value(gs)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		gs,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Gs)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Gs)
}

func (RegistersSuite) TestSs(t *testing.T) {
	registers := NewRegisterSet()

	ss, ok := registers.RegisterByName("ss")
	expect.True(t, ok)
	expect.Equal(t, 52, ss.DwarfId)

	state := RegisterState{}
	state.gpr.Ss = 0x0102030405060708

	val := state.Value(ss)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		ss,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Ss)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Ss)
}

func (RegistersSuite) TestDs(t *testing.T) {
	registers := NewRegisterSet()

	ds, ok := registers.RegisterByName("ds")
	expect.True(t, ok)
	expect.Equal(t, 53, ds.DwarfId)

	state := RegisterState{}
	state.gpr.Ds = 0x0102030405060708

	val := state.Value(ds)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		ds,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Ds)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Ds)
}

func (RegistersSuite) TestEs(t *testing.T) {
	registers := NewRegisterSet()

	es, ok := registers.RegisterByName("es")
	expect.True(t, ok)
	expect.Equal(t, 50, es.DwarfId)

	state := RegisterState{}
	state.gpr.Es = 0x0102030405060708

	val := state.Value(es)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		es,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.gpr.Es)
	expect.Equal(t, 0x1020304050607080, newState.gpr.Es)
}

func (RegistersSuite) TestFcw(t *testing.T) {
	registers := NewRegisterSet()

	fcw, ok := registers.RegisterByName("fcw")
	expect.True(t, ok)
	expect.Equal(t, 65, fcw.DwarfId)

	state := RegisterState{}
	state.fpr.Cwd = 0x0102

	val := state.Value(fcw)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0102, u16.Value)

	newState, err := state.WithValue(fcw, Uint16Value(0x1020))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102, state.fpr.Cwd)
	expect.Equal(t, 0x1020, newState.fpr.Cwd)
}

func (RegistersSuite) TestFsw(t *testing.T) {
	registers := NewRegisterSet()

	fsw, ok := registers.RegisterByName("fsw")
	expect.True(t, ok)
	expect.Equal(t, 66, fsw.DwarfId)

	state := RegisterState{}
	state.fpr.Swd = 0x0102

	val := state.Value(fsw)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0102, u16.Value)

	newState, err := state.WithValue(fsw, Uint16Value(0x1020))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102, state.fpr.Swd)
	expect.Equal(t, 0x1020, newState.fpr.Swd)
}

func (RegistersSuite) TestFtw(t *testing.T) {
	registers := NewRegisterSet()

	ftw, ok := registers.RegisterByName("ftw")
	expect.True(t, ok)

	state := RegisterState{}
	state.fpr.Ftw = 0x0102

	val := state.Value(ftw)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0102, u16.Value)

	newState, err := state.WithValue(ftw, Uint16Value(0x1020))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102, state.fpr.Ftw)
	expect.Equal(t, 0x1020, newState.fpr.Ftw)
}

func (RegistersSuite) TestFop(t *testing.T) {
	registers := NewRegisterSet()

	fop, ok := registers.RegisterByName("fop")
	expect.True(t, ok)

	state := RegisterState{}
	state.fpr.Fop = 0x0102

	val := state.Value(fop)
	u16, ok := val.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0102, u16.Value)

	newState, err := state.WithValue(fop, Uint16Value(0x1020))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102, state.fpr.Fop)
	expect.Equal(t, 0x1020, newState.fpr.Fop)
}

func (RegistersSuite) TestFrip(t *testing.T) {
	registers := NewRegisterSet()

	frip, ok := registers.RegisterByName("frip")
	expect.True(t, ok)

	state := RegisterState{}
	state.fpr.Rip = 0x0102030405060708

	val := state.Value(frip)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		frip,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.fpr.Rip)
	expect.Equal(t, 0x1020304050607080, newState.fpr.Rip)
}

func (RegistersSuite) TestFrdp(t *testing.T) {
	registers := NewRegisterSet()

	frdp, ok := registers.RegisterByName("frdp")
	expect.True(t, ok)

	state := RegisterState{}
	state.fpr.Rdp = 0x0102030405060708

	val := state.Value(frdp)
	u64, ok := val.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u64.Value)

	newState, err := state.WithValue(
		frdp,
		Uint64Value(0x1020304050607080))
	expect.Nil(t, err)
	expect.Equal(t, 0x0102030405060708, state.fpr.Rdp)
	expect.Equal(t, 0x1020304050607080, newState.fpr.Rdp)
}

func (RegistersSuite) TestMxcsr(t *testing.T) {
	registers := NewRegisterSet()

	mxcsr, ok := registers.RegisterByName("mxcsr")
	expect.True(t, ok)

	state := RegisterState{}
	state.fpr.Mxcsr = 0x01020304

	val := state.Value(mxcsr)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x01020304, u32.Value)

	newState, err := state.WithValue(
		mxcsr,
		Uint32Value(0x10203040))
	expect.Nil(t, err)
	expect.Equal(t, 0x01020304, state.fpr.Mxcsr)
	expect.Equal(t, 0x10203040, newState.fpr.Mxcsr)
}

func (RegistersSuite) TestMxcrMask(t *testing.T) {
	registers := NewRegisterSet()

	mxcrmask, ok := registers.RegisterByName("mxcrmask")
	expect.True(t, ok)

	state := RegisterState{}
	state.fpr.MxcrMask = 0x01020304

	val := state.Value(mxcrmask)
	u32, ok := val.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x01020304, u32.Value)

	newState, err := state.WithValue(
		mxcrmask,
		Uint32Value(0x10203040))
	expect.Nil(t, err)
	expect.Equal(t, 0x01020304, state.fpr.MxcrMask)
	expect.Equal(t, 0x10203040, newState.fpr.MxcrMask)
}

func TestStMm(t *testing.T) {
	registers := NewRegisterSet()

	for i := 0; i < 8; i++ {
		stName := fmt.Sprintf("st%d", i)

		st, ok := registers.RegisterByName(stName)
		expect.True(t, ok)

		mmName := fmt.Sprintf("mm%d", i)

		mm, ok := registers.RegisterByName(mmName)
		expect.True(t, ok)

		state := RegisterState{}

		lowIdx := 2 * i
		highIdx := 2*i + 1

		low := uint64((i + 1) * 100)
		high := ^low

		state.fpr.StSpace[lowIdx] = low
		state.fpr.StSpace[highIdx] = high

		val := state.Value(st)
		u128, ok := val.(Uint128)
		expect.True(t, ok)
		expect.Equal(t, low, u128.Low)
		expect.Equal(t, high, u128.High)

		val = state.Value(mm)
		u128, ok = val.(Uint128)
		expect.True(t, ok)
		expect.Equal(t, low, u128.Low)
		expect.Equal(t, high, u128.High)

		newLow := low + 1
		newHigh := ^newLow

		newState, err := state.WithValue(st, Uint128Value(newHigh, newLow))
		expect.Nil(t, err)
		expect.Equal(t, low, state.fpr.StSpace[lowIdx])
		expect.Equal(t, high, state.fpr.StSpace[highIdx])
		expect.Equal(t, newLow, newState.fpr.StSpace[lowIdx])
		expect.Equal(t, newHigh, newState.fpr.StSpace[highIdx])

		newLow += 1
		newHigh = ^newHigh

		newState, err = state.WithValue(mm, Uint128Value(newHigh, newLow))
		expect.Nil(t, err)
		expect.Equal(t, low, state.fpr.StSpace[lowIdx])
		expect.Equal(t, high, state.fpr.StSpace[highIdx])
		expect.Equal(t, newLow, newState.fpr.StSpace[lowIdx])
		expect.Equal(t, newHigh, newState.fpr.StSpace[highIdx])

		state.fpr.StSpace[lowIdx] = 0xabc
		state.fpr.StSpace[highIdx] = 0xdef

		newLow = 0x0102030405060708
		newState, err = state.WithValue(
			mm,
			Float64Value(math.Float64frombits(newLow)))
		expect.Nil(t, err)
		expect.Equal(t, 0xabc, state.fpr.StSpace[lowIdx])
		expect.Equal(t, 0xdef, state.fpr.StSpace[highIdx])
		expect.Equal(t, newLow, newState.fpr.StSpace[lowIdx])
		expect.Equal(t, 0, newState.fpr.StSpace[highIdx])

		newLow = 0x01020304
		newState, err = state.WithValue(
			st,
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
		name := fmt.Sprintf("xmm%d", i)

		xmm, ok := registers.RegisterByName(name)
		expect.True(t, ok)

		state := RegisterState{}

		lowIdx := 2 * i
		highIdx := 2*i + 1

		low := uint64((i + 1) * 100)
		high := ^low

		state.fpr.XmmSpace[lowIdx] = low
		state.fpr.XmmSpace[highIdx] = high

		val := state.Value(xmm)
		u128, ok := val.(Uint128)
		expect.True(t, ok)
		expect.Equal(t, low, u128.Low)
		expect.Equal(t, high, u128.High)

		newLow := low + 1
		newHigh := ^newLow

		newState, err := state.WithValue(xmm, Uint128Value(newHigh, newLow))
		expect.Nil(t, err)
		expect.Equal(t, low, state.fpr.XmmSpace[lowIdx])
		expect.Equal(t, newLow, newState.fpr.XmmSpace[lowIdx])
		expect.Equal(t, high, state.fpr.XmmSpace[highIdx])
		expect.Equal(t, newHigh, newState.fpr.XmmSpace[highIdx])

		state.fpr.XmmSpace[lowIdx] = 0xabc
		state.fpr.XmmSpace[highIdx] = 0xdef

		newLow = 0x0102030405060708
		newState, err = state.WithValue(
			xmm,
			Float64Value(math.Float64frombits(newLow)))
		expect.Nil(t, err)
		expect.Equal(t, 0xabc, state.fpr.XmmSpace[lowIdx])
		expect.Equal(t, 0xdef, state.fpr.XmmSpace[highIdx])
		expect.Equal(t, newLow, newState.fpr.XmmSpace[lowIdx])
		expect.Equal(t, 0, newState.fpr.XmmSpace[highIdx])

		newLow = 0x01020304
		newState, err = state.WithValue(
			xmm,
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

		dr, ok := registers.RegisterByName(name)
		expect.True(t, ok)

		value := uint64((i + 1) * 10)

		state := RegisterState{}
		state.dr[i] = uintptr(value)

		val := state.Value(dr)
		u64, ok := val.(Uint64)
		expect.True(t, ok)
		expect.Equal(t, value, u64.Value)

		newState, err := state.WithValue(dr, Uint64Value(value+1))
		if i == 4 || i == 5 {
			expect.Error(t, err, "read-only")
		} else {
			expect.Nil(t, err)
			expect.Equal(t, uintptr(value), state.dr[i])
			expect.Equal(t, uintptr(value+1), newState.dr[i])
		}
	}
}

func (RegistersSuite) TestParseFloat32Value(t *testing.T) {
	registers := NewRegisterSet()

	reg8, ok := registers.RegisterByName("al")
	expect.True(t, ok)

	value, err := reg8.ParseValue("f:32.125")
	expect.Nil(t, err)

	f, ok := value.(Float32)
	expect.True(t, ok)
	expect.Equal(t, 32.125, f)

	_, err = reg8.ParseValue("f:bad")
	expect.Error(t, err, "failed to parse float32")
}

func (RegistersSuite) TestParseFloat64Value(t *testing.T) {
	registers := NewRegisterSet()

	reg8, ok := registers.RegisterByName("al")
	expect.True(t, ok)

	value, err := reg8.ParseValue("d:64.125")
	expect.Nil(t, err)

	d, ok := value.(Float64)
	expect.True(t, ok)
	expect.Equal(t, 64.125, d)

	_, err = reg8.ParseValue("d:bad")
	expect.Error(t, err, "failed to parse float64")
}

func (RegistersSuite) TestParseInt64Value(t *testing.T) {
	registers := NewRegisterSet()

	reg128, ok := registers.RegisterByName("xmm0")
	expect.True(t, ok)

	reg64, ok := registers.RegisterByName("r10")
	expect.True(t, ok)

	value, err := reg128.ParseValue("i:-0x0102030405060708")
	expect.Nil(t, err)

	i, ok := value.(Int64)
	expect.True(t, ok)
	expect.Equal(t, -0x0102030405060708, i.Value)

	value, err = reg64.ParseValue("i:0x1020304050607080")
	expect.Nil(t, err)

	i, ok = value.(Int64)
	expect.True(t, ok)
	expect.Equal(t, 0x1020304050607080, i.Value)

	_, err = reg128.ParseValue("i:-0x010203040506070809")
	expect.Error(t, err, "failed to parse int")
}

func (RegistersSuite) TestParseInt32Value(t *testing.T) {
	registers := NewRegisterSet()

	reg32, ok := registers.RegisterByName("eax")
	expect.True(t, ok)

	value, err := reg32.ParseValue("i:-0x01020304")
	expect.Nil(t, err)

	i, ok := value.(Int32)
	expect.True(t, ok)
	expect.Equal(t, -0x01020304, i.Value)

	_, err = reg32.ParseValue("i:-0x0102030405")
	expect.Error(t, err, "failed to parse int")
}

func (RegistersSuite) TestParseInt16Value(t *testing.T) {
	registers := NewRegisterSet()

	reg16, ok := registers.RegisterByName("ax")
	expect.True(t, ok)

	value, err := reg16.ParseValue("i:-0x0102")
	expect.Nil(t, err)

	i, ok := value.(Int16)
	expect.True(t, ok)
	expect.Equal(t, -0x0102, i.Value)

	_, err = reg16.ParseValue("i:-0x010203")
	expect.Error(t, err, "failed to parse int")
}

func (RegistersSuite) TestParseInt8Value(t *testing.T) {
	registers := NewRegisterSet()

	reg8, ok := registers.RegisterByName("al")
	expect.True(t, ok)

	value, err := reg8.ParseValue("i:-0x01")
	expect.Nil(t, err)

	i, ok := value.(Int8)
	expect.True(t, ok)
	expect.Equal(t, -0x01, i.Value)

	_, err = reg8.ParseValue("i:-0x0102")
	expect.Error(t, err, "failed to parse int")
}

func (RegistersSuite) TestParseUint64Value(t *testing.T) {
	registers := NewRegisterSet()

	reg128, ok := registers.RegisterByName("xmm0")
	expect.True(t, ok)

	reg64, ok := registers.RegisterByName("r10")
	expect.True(t, ok)

	value, err := reg128.ParseValue("0x0102030405060708")
	expect.Nil(t, err)

	i, ok := value.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, i.Value)

	value, err = reg64.ParseValue("0x1020304050607080")
	expect.Nil(t, err)

	i, ok = value.(Uint64)
	expect.True(t, ok)
	expect.Equal(t, 0x1020304050607080, i.Value)

	_, err = reg128.ParseValue("0x010203040506070809")
	expect.Error(t, err, "failed to parse uint")
}

func (RegistersSuite) TestParseUint32Value(t *testing.T) {
	registers := NewRegisterSet()

	reg32, ok := registers.RegisterByName("eax")
	expect.True(t, ok)

	value, err := reg32.ParseValue("0x01020304")
	expect.Nil(t, err)

	i, ok := value.(Uint32)
	expect.True(t, ok)
	expect.Equal(t, 0x01020304, i.Value)

	_, err = reg32.ParseValue("0x0102030405")
	expect.Error(t, err, "failed to parse uint")
}

func (RegistersSuite) TestParseUint16Value(t *testing.T) {
	registers := NewRegisterSet()

	reg16, ok := registers.RegisterByName("ax")
	expect.True(t, ok)

	value, err := reg16.ParseValue("0x0102")
	expect.Nil(t, err)

	i, ok := value.(Uint16)
	expect.True(t, ok)
	expect.Equal(t, 0x0102, i.Value)

	_, err = reg16.ParseValue("0x010203")
	expect.Error(t, err, "failed to parse uint")
}

func (RegistersSuite) TestParseUint8Value(t *testing.T) {
	registers := NewRegisterSet()

	reg8, ok := registers.RegisterByName("al")
	expect.True(t, ok)

	value, err := reg8.ParseValue("0x01")
	expect.Nil(t, err)

	i, ok := value.(Uint8)
	expect.True(t, ok)
	expect.Equal(t, 0x01, i.Value)

	_, err = reg8.ParseValue("0x0102")
	expect.Error(t, err, "failed to parse uint")
}

func (RegistersSuite) TestParseUint128Value(t *testing.T) {
	registers := NewRegisterSet()

	reg8, ok := registers.RegisterByName("al")
	expect.True(t, ok)

	value, err := reg8.ParseValue("0x1:2")
	expect.Nil(t, err)

	u, ok := value.(Uint128)
	expect.True(t, ok)
	expect.Equal(t, 1, u.High)
	expect.Equal(t, 2, u.Low)

	value, err = reg8.ParseValue("0x0102030405060708:0x1020304050607080")
	expect.Nil(t, err)

	u, ok = value.(Uint128)
	expect.True(t, ok)
	expect.Equal(t, 0x0102030405060708, u.High)
	expect.Equal(t, 0x1020304050607080, u.Low)

	_, err = reg8.ParseValue("-1:0x1020304050607080")
	expect.Error(t, err, "failed to parse uint128 high word (-1)")

	_, err = reg8.ParseValue("0x0102030405060708:-2")
	expect.Error(t, err, "failed to parse uint128 low word (-2)")
}
