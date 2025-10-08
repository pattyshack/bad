package dwarf

import (
	"encoding/binary"
	"fmt"
	"io"
)

type LocationKind string

const (
	// Address and contents of the variable are unknown (e.g., due to compiler
	// optimization)
	UnavailableLocation = LocationKind("unavailable")

	// Value is interpreted as virtual address
	AddressLocation = LocationKind("address")

	// Value is interpreted as dwarf RegisterId
	RegisterLocation = LocationKind("register")

	// No real storage. Value is interpreted as implicit literal
	ImplicitLiteralLocation = LocationKind("implicit literal")

	// No real storage. Data slice
	ImplicitDataLocation = LocationKind("implicit data")
)

type LocationChunk struct {
	Kind LocationKind

	Value uint64
	Data  []byte

	// NOTE: when BitSize is zero, the entire value is used.
	BitSize   uint64
	BitOffset uint64
}

// Empty slice indicates empty result (address/contents of the variable are
// unknown).  Single element slice indicates simple location.  Multi-elements
// slice indicates composite location.
type Location []LocationChunk

type ExpressionContext interface {
	ByteOrder() binary.ByteOrder

	LoadBias() uint64 // virtual address

	CurrentFunctionEntry() *DebugInfoEntry

	ProgramCounter() uint64 // virtual address

	RegisterValue(id RegisterId) (uint64, error)

	ReadMemory(virtualAddress uint64, out []byte) (int, error)

	CanonicalFrameAddress() (uint64, error) // virtual address
}

func EvaluateExpression(
	context ExpressionContext,
	inFrameInfo bool,
	instructions []byte,
	initializeStackWithCFA bool,
) (
	Location,
	error,
) {
	state := &expressionState{
		context:     context,
		Cursor:      NewCursor(context.ByteOrder(), instructions),
		inFrameInfo: inFrameInfo,
	}

	if initializeStackWithCFA {
		cfa, err := context.CanonicalFrameAddress()
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate expression: %w", err)
		}
		state.push(cfa)
	}

	for !state.HasReachedEnd() {
		err := state.executeInstruction()
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate expression: %w", err)
		}
	}

	if len(state.result) == 0 {
		if len(state.stack) > 0 {
			err := state.endSimpleChunk()
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate expression: %w", err)
			}
		}

		if state.currentChunk != nil {
			state.result = append(state.result, *state.currentChunk)
			state.currentChunk = nil
		}
	}

	if state.currentChunk != nil {
		return nil, fmt.Errorf(
			"failed to evaluate expression. composite chunk not terminated")
	}

	if len(state.stack) > 0 {
		return nil, fmt.Errorf(
			"failed to evaluate expression. stack not empty after evaluation")
	}

	return state.result, nil
}

type expressionState struct {
	context ExpressionContext
	*Cursor

	inFrameInfo bool

	stack               []uint64
	stackValueIsLiteral bool // false for address

	currentChunk *LocationChunk

	result Location
}

func (state *expressionState) executeInstruction() error {
	_opCode, err := state.U8()
	if err != nil {
		return err
	}
	opCode := Operation(_opCode)

	if opCode == DW_OP_addr ||
		DW_OP_const1u <= opCode && opCode <= DW_OP_consts ||
		DW_OP_lit0 <= opCode && opCode <= DW_OP_lit31 ||
		opCode == DW_OP_call_frame_cfa {

		return state.pushConst(opCode)
	}

	if DW_OP_breg0 <= opCode && opCode <= DW_OP_breg31 || opCode == DW_OP_bregx {
		return state.breg(opCode)
	}

	if DW_OP_reg0 <= opCode && opCode <= DW_OP_reg31 || opCode == DW_OP_regx {
		return state.reg(opCode)
	}

	switch opCode {
	case DW_OP_deref, DW_OP_deref_size:
		return state.deref(opCode)

	case DW_OP_dup, DW_OP_pick, DW_OP_over:
		return state.dup(opCode)
	case DW_OP_drop:
		_, err := state.pop()
		return err
	case DW_OP_swap:
		return state.swap()
	case DW_OP_rot:
		return state.rot()

	case DW_OP_abs:
		return state.abs()
	case DW_OP_neg:
		return state.neg()
	case DW_OP_not:
		return state.not()
	case DW_OP_plus_uconst:
		return state.plusConst()

	case DW_OP_plus:
		return state.plus()
	case DW_OP_minus:
		return state.minus()
	case DW_OP_mul:
		return state.mul()
	case DW_OP_div:
		return state.div()
	case DW_OP_mod:
		return state.mod()
	case DW_OP_shl:
		return state.shl()
	case DW_OP_shr:
		return state.shr()
	case DW_OP_shra:
		return state.shra()
	case DW_OP_and:
		return state.and()
	case DW_OP_or:
		return state.or()
	case DW_OP_xor:
		return state.xor()

	case DW_OP_eq:
		return state.eq()
	case DW_OP_ne:
		return state.ne()
	case DW_OP_ge:
		return state.ge()
	case DW_OP_gt:
		return state.gt()
	case DW_OP_le:
		return state.le()
	case DW_OP_lt:
		return state.lt()

	case DW_OP_fbreg:
		return state.fbreg()

	case DW_OP_stack_value:
		state.stackValueIsLiteral = true
		return nil
	case DW_OP_implicit_value:
		return state.implicitData()
	case DW_OP_piece, DW_OP_bit_piece:
		return state.compositeChunk(opCode)

	case DW_OP_nop:
		// do nothing
	case DW_OP_skip: // aka unconditional jump
		return state.skip()
	case DW_OP_bra:
		return state.bra()
	}

	return fmt.Errorf("unsupported op code %s", opCode)
}

func (state *expressionState) push(val uint64) {
	state.stack = append(state.stack, val)
}

func (state *expressionState) pop() (uint64, error) {
	if len(state.stack) == 0 {
		return 0, fmt.Errorf("cannot pop empty stack")
	}

	val := state.stack[len(state.stack)-1]
	state.stack = state.stack[:len(state.stack)-1]
	return val, nil
}

func (state *expressionState) pushConst(opCode Operation) error {
	value := uint64(opCode - DW_OP_lit0)
	switch opCode {
	case DW_OP_addr:
		n, err := state.U64()
		if err != nil {
			return err
		}
		value = n + state.context.LoadBias()
	case DW_OP_const1u:
		n, err := state.U8()
		if err != nil {
			return err
		}
		value = uint64(n)
	case DW_OP_const1s:
		n, err := state.S8()
		if err != nil {
			return err
		}
		value = uint64(n)
	case DW_OP_const2u:
		n, err := state.U16()
		if err != nil {
			return err
		}
		value = uint64(n)
	case DW_OP_const2s:
		n, err := state.S16()
		if err != nil {
			return err
		}
		value = uint64(n)
	case DW_OP_const4u:
		n, err := state.U32()
		if err != nil {
			return err
		}
		value = uint64(n)
	case DW_OP_const4s:
		n, err := state.S32()
		if err != nil {
			return err
		}
		value = uint64(n)
	case DW_OP_const8u:
		n, err := state.U64()
		if err != nil {
			return err
		}
		value = n
	case DW_OP_const8s:
		n, err := state.S64()
		if err != nil {
			return err
		}
		value = uint64(n)
	case DW_OP_constu:
		n, err := state.ULEB128(64)
		if err != nil {
			return err
		}
		value = n
	case DW_OP_consts:
		n, err := state.SLEB128(64)
		if err != nil {
			return err
		}
		value = uint64(n)
	case DW_OP_call_frame_cfa:
		n, err := state.context.CanonicalFrameAddress()
		if err != nil {
			return err
		}
		value = n
	}

	state.push(value)
	return nil
}

func (state *expressionState) deref(opCode Operation) error {
	addr, err := state.pop()
	if err != nil {
		return err
	}

	size := 8
	if opCode == DW_OP_deref_size {
		n, err := state.U8()
		if err != nil {
			return err
		}
		if n > 8 {
			return fmt.Errorf("invalid deref size %d", n)
		}

		size = int(n)
	}

	bytes := make([]byte, 8)

	n, err := state.context.ReadMemory(addr, bytes[:size])
	if err != nil {
		return err
	}
	if n != size {
		panic("should never happen")
	}

	value := uint64(0)
	n, err = binary.Decode(bytes, state.ByteOrder, &value)
	if err != nil {
		return err
	}
	if n != 8 {
		panic("should never happen")
	}

	state.push(value)
	return nil
}

func (state *expressionState) dup(opCode Operation) error {
	idx := len(state.stack) - 1 // dup
	if opCode == DW_OP_over {
		idx = len(state.stack) - 2
	} else if opCode == DW_OP_pick {
		n, err := state.U8()
		if err != nil {
			return err
		}

		idx = len(state.stack) - int(n)
	}

	if idx < 0 {
		return fmt.Errorf("invalid %s. out of bound", opCode)
	}

	state.push(state.stack[idx])
	return nil
}

func (state *expressionState) swap() error {
	top, err := state.pop()
	if err != nil {
		return err
	}

	bot, err := state.pop()
	if err != nil {
		return err
	}

	state.push(top)
	state.push(bot)
	return nil
}

func (state *expressionState) rot() error {
	top, err := state.pop()
	if err != nil {
		return err
	}

	mid, err := state.pop()
	if err != nil {
		return err
	}

	bot, err := state.pop()
	if err != nil {
		return err
	}

	// [... bot mid top] -> [... top bot mid]
	state.push(top)
	state.push(bot)
	state.push(mid)
	return nil
}

func (state *expressionState) breg(opCode Operation) error {
	var regId RegisterId
	if opCode == DW_OP_bregx {
		id, err := state.ULEB128(64)
		if err != nil {
			return err
		}

		regId = RegisterId(id)
	} else {
		regId = RegisterId(opCode - DW_OP_breg0)
	}

	value, err := state.context.RegisterValue(regId)
	if err != nil {
		return err
	}

	offset, err := state.SLEB128(64)
	if err != nil {
		return err
	}

	state.push(uint64(int64(value) + offset))
	return nil
}

func (state *expressionState) fbreg() error {
	entry := state.context.CurrentFunctionEntry()
	if entry == nil {
		return fmt.Errorf("current function debug info entry unavailable")
	}

	location, err := entry.EvaluateLocation(
		DW_AT_frame_base,
		state.context,
		true,  // in frame info
		false) // initialize stack with cfa
	if err != nil {
		return err
	}

	if len(location) != 1 || location[0].Kind != AddressLocation {
		return fmt.Errorf("unsupported frame base location")
	}

	offset, err := state.SLEB128(64)
	if err != nil {
		return err
	}

	state.push(uint64(int64(location[0].Value) + offset))
	return nil
}

func (state *expressionState) reg(opCode Operation) error {
	var regId RegisterId
	if opCode == DW_OP_regx {
		id, err := state.ULEB128(64)
		if err != nil {
			return err
		}

		regId = RegisterId(id)
	} else {
		regId = RegisterId(opCode - DW_OP_reg0)
	}

	if state.inFrameInfo {
		value, err := state.context.RegisterValue(regId)
		if err != nil {
			return err
		}
		state.push(value)
	} else {
		if state.currentChunk != nil {
			// should have been terminated by DW_OP_piece or DW_OP_bit_piece
			return fmt.Errorf("composite location chunk not terminated")
		}

		state.currentChunk = &LocationChunk{
			Kind:  RegisterLocation,
			Value: uint64(regId),
		}
	}

	return nil
}

func (state *expressionState) skip() error {
	offset, err := state.S16()
	if err != nil {
		return err
	}

	_, err = state.Seek(int(offset), io.SeekCurrent)
	return err
}

func (state *expressionState) bra() error {
	offset, err := state.S16()
	if err != nil {
		return err
	}

	predicate, err := state.pop()
	if err != nil {
		return err
	}

	if predicate != 0 {
		_, err := state.Seek(int(offset), io.SeekCurrent)
		return err
	}

	return nil
}

func (state *expressionState) abs() error {
	n, err := state.pop()
	if err != nil {
		return err
	}

	value := int64(n)
	if n < 0 {
		value = -value
	}

	state.push(uint64(value))
	return nil
}

func (state *expressionState) neg() error {
	value, err := state.pop()
	if err != nil {
		return err
	}

	state.push(uint64(-int64(value)))
	return nil
}

func (state *expressionState) not() error {
	value, err := state.pop()
	if err != nil {
		return err
	}

	state.push(^value)
	return nil
}

func (state *expressionState) plusConst() error {
	value, err := state.pop()
	if err != nil {
		return err
	}

	n, err := state.ULEB128(64)
	if err != nil {
		return err
	}

	state.push(value + n)
	return nil
}

func (state *expressionState) plus() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	state.push(lhs + rhs)
	return nil
}

func (state *expressionState) minus() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	state.push(lhs - rhs)
	return nil
}

func (state *expressionState) mul() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	state.push(lhs * rhs)
	return nil
}

func (state *expressionState) div() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	state.push(uint64(int64(lhs) / int64(rhs)))
	return nil
}

func (state *expressionState) mod() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	state.push(lhs % rhs)
	return nil
}

func (state *expressionState) shl() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	state.push(lhs << rhs)
	return nil
}

func (state *expressionState) shr() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	state.push(lhs >> rhs)
	return nil
}

func (state *expressionState) shra() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	state.push(uint64(int64(lhs) >> rhs))
	return nil
}

func (state *expressionState) and() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	state.push(lhs & rhs)
	return nil
}

func (state *expressionState) or() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	state.push(lhs | rhs)
	return nil
}

func (state *expressionState) xor() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	state.push(lhs ^ rhs)
	return nil
}

func (state *expressionState) eq() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	val := uint64(0)
	if int64(lhs) == int64(rhs) {
		val = 1
	}

	state.push(val)
	return nil
}

func (state *expressionState) ne() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	val := uint64(0)
	if int64(lhs) != int64(rhs) {
		val = 1
	}

	state.push(val)
	return nil
}

func (state *expressionState) ge() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	val := uint64(0)
	if int64(lhs) >= int64(rhs) {
		val = 1
	}

	state.push(val)
	return nil
}

func (state *expressionState) gt() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	val := uint64(0)
	if int64(lhs) > int64(rhs) {
		val = 1
	}

	state.push(val)
	return nil
}

func (state *expressionState) le() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	val := uint64(0)
	if int64(lhs) <= int64(rhs) {
		val = 1
	}

	state.push(val)
	return nil
}

func (state *expressionState) lt() error {
	rhs, err := state.pop()
	if err != nil {
		return err
	}

	lhs, err := state.pop()
	if err != nil {
		return err
	}

	val := uint64(0)
	if int64(lhs) < int64(rhs) {
		val = 1
	}

	state.push(val)
	return nil
}

func (state *expressionState) implicitData() error {
	length, err := state.ULEB128(32)
	if err != nil {
		return err
	}

	data, err := state.Bytes(int(length))
	if err != nil {
		return err
	}

	if state.currentChunk != nil {
		// should have been terminated by DW_OP_piece or DW_OP_bit_piece
		return fmt.Errorf("composite location chunk not terminated")
	}

	state.currentChunk = &LocationChunk{
		Kind: ImplicitDataLocation,
		Data: data,
	}

	return nil
}

func (state *expressionState) endSimpleChunk() error {
	if state.currentChunk != nil {
		return nil
	}

	// XXX: do we really need this case?
	if len(state.stack) == 0 {
		state.currentChunk = &LocationChunk{
			Kind: UnavailableLocation,
		}
		return nil
	}

	value, err := state.pop()
	if err != nil {
		return err
	}

	kind := AddressLocation
	if state.stackValueIsLiteral {
		kind = ImplicitLiteralLocation
	}
	state.stackValueIsLiteral = false

	state.currentChunk = &LocationChunk{
		Kind:  kind,
		Value: value,
	}

	return err
}

func (state *expressionState) compositeChunk(opCode Operation) error {
	err := state.endSimpleChunk()
	if err != nil {
		return err
	}

	bitSize := uint64(0)
	offset := uint64(0)
	if opCode == DW_OP_piece {
		byteSize, err := state.ULEB128(64)
		if err != nil {
			return err
		}

		bitSize = 8 * byteSize
	} else { // DW_OP_bit_piece
		var err error
		bitSize, err = state.ULEB128(64)
		if err != nil {
			return err
		}

		offset, err = state.ULEB128(64)
		if err != nil {
			return err
		}
	}

	chunkBitSize := 64
	if state.currentChunk.Kind == ImplicitDataLocation {
		chunkBitSize = 8 * len(state.currentChunk.Data)
	}

	if bitSize > uint64(chunkBitSize) {
		return fmt.Errorf(
			"invalid location chunk bit size (%d > %d)",
			bitSize,
			chunkBitSize)
	}

	state.currentChunk.BitSize = bitSize
	state.currentChunk.BitOffset = offset

	state.result = append(state.result, *state.currentChunk)
	state.currentChunk = nil

	return nil
}
