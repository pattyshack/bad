package dwarf

import (
	"fmt"

	"github.com/pattyshack/bad/elf"
)

type RegisterId int

const (
	DW_CFA_advance_loc = 0x40
	DW_CFA_offset      = 0x80
	DW_CFA_restore     = 0xc0

	DW_CFA_nop                = 0x00
	DW_CFA_set_loc            = 0x01
	DW_CFA_advance_loc1       = 0x02
	DW_CFA_advance_loc2       = 0x03
	DW_CFA_advance_loc4       = 0x04
	DW_CFA_offset_extended    = 0x05
	DW_CFA_restore_extended   = 0x06
	DW_CFA_undefined          = 0x07
	DW_CFA_same_value         = 0x08
	DW_CFA_register           = 0x09
	DW_CFA_remember_state     = 0x0a
	DW_CFA_restore_state      = 0x0b
	DW_CFA_def_cfa            = 0x0c
	DW_CFA_def_cfa_register   = 0x0d
	DW_CFA_def_cfa_offset     = 0x0e
	DW_CFA_def_cfa_expression = 0x0f
	DW_CFA_expression         = 0x10
	DW_CFA_offset_extended_sf = 0x11
	DW_CFA_def_cfa_sf         = 0x12
	DW_CFA_def_cfa_offset_sf  = 0x13
	DW_CFA_val_offset         = 0x14
	DW_CFA_val_offset_sf      = 0x15
	DW_CFA_val_expression     = 0x16
	DW_CFA_lo_user            = 0x1c
	DW_CFA_hi_user            = 0x3f
)

type RegisterRuleKind string

const (
	// Unable to restore previous register
	UndefinedRule = RegisterRuleKind("undefined")

	// The previous register's value is currently in a different current register.
	InRegisterRule = RegisterRuleKind("register")

	// The previous register's value is the same as the current register.
	SameValueRule = RegisterRuleKind("same value")

	// The previous register's value is saved in memory at CFA + offset
	OffsetRule = RegisterRuleKind("offset")

	// The previous register's value is CFA + offset
	ValueOffsetRule = RegisterRuleKind("value offset")

	// The previous register's value is saved in memory at address computed by
	// executing the dwarf expression.
	ExpressionRule = RegisterRuleKind("expression")

	// The previous register's value is the value computed by executing the
	// dwarf expression.
	ValueExpressionRule = RegisterRuleKind("value expression")

	// CFA = current RegisterId's value + offset
	CFARegisterOffsetRule = RegisterRuleKind("cfa register offset")
)

type RegisterRule struct {
	Kind RegisterRuleKind

	RegisterId // used by RegisterRule and CFARegisterOffsetRule

	Offset int64 // used by OffsetRule, ValueOffsetRule, and CFARegisterOffsetRule

	// TODO expression data
}

type UnwindRules struct {
	CanonicalFrameAddress RegisterRule
	Registers             map[RegisterId]RegisterRule
}

func newUnwindRules() *UnwindRules {
	return &UnwindRules{
		Registers: map[RegisterId]RegisterRule{},
	}
}

func (rules *UnwindRules) Copy() *UnwindRules {
	registers := make(map[RegisterId]RegisterRule, len(rules.Registers))
	for id, rule := range rules.Registers {
		registers[id] = rule
	}

	return &UnwindRules{
		CanonicalFrameAddress: rules.CanonicalFrameAddress,
		Registers:             registers,
	}
}

func (rules *UnwindRules) SetRegisterRule(
	id RegisterId,
	rule RegisterRule,
) {
	rules.Registers[id] = rule
}

func (rules *UnwindRules) GetRegisterRule(
	id RegisterId,
) (
	RegisterRule,
	error,
) {
	rule, ok := rules.Registers[id]
	if !ok {
		return RegisterRule{}, fmt.Errorf("register rule for %d not found", id)
	}

	return rule, nil
}

func (rules *UnwindRules) SetCFARegisterOffset(id RegisterId, offset int64) {
	rules.CanonicalFrameAddress.Kind = CFARegisterOffsetRule
	rules.CanonicalFrameAddress.RegisterId = id
	rules.CanonicalFrameAddress.Offset = offset
}

type cfiState struct {
	*FrameDescriptionEntry

	location elf.FileAddress

	cieRules *UnwindRules
	stack    []*UnwindRules
}

func computeUnwindRules(
	fde *FrameDescriptionEntry,
	pc elf.FileAddress,
) (
	*UnwindRules,
	error,
) {
	state := &cfiState{
		FrameDescriptionEntry: fde,
		cieRules:              nil,
		stack:                 []*UnwindRules{newUnwindRules()},
	}

	decode := newCIEInstructionDecoder(state)
	for !decode.HasReachedEnd() {
		err := state.executeInstruction(decode)
		if err != nil {
			return nil, fmt.Errorf("failed to execute cie instruction: %w", err)
		}
	}

	state.saveCIERules()

	state.location = state.AddressRange.Low
	decode = newFDEInstructionDecoder(state)
	for !decode.HasReachedEnd() && state.location <= pc {
		err := state.executeInstruction(decode)
		if err != nil {
			return nil, fmt.Errorf("failed to execute fde instruction: %w", err)
		}
	}

	return state.top()
}

func (state *cfiState) top() (*UnwindRules, error) {
	if len(state.stack) == 0 {
		return nil, fmt.Errorf("no unwind rules on stack")
	}

	return state.stack[len(state.stack)-1], nil
}

func (state *cfiState) push() error {
	top, err := state.top()
	if err != nil {
		return err
	}

	state.stack = append(state.stack, top.Copy())
	return nil
}

func (state *cfiState) pop() error {
	if len(state.stack) == 0 {
		return fmt.Errorf("no unwind rules on stack")
	}

	state.stack = state.stack[:len(state.stack)-1]
	return nil
}

func (state *cfiState) setCFARegisterOffset(id RegisterId, offset int64) error {
	top, err := state.top()
	if err != nil {
		return err
	}

	top.SetCFARegisterOffset(id, offset)
	return nil
}

func (state *cfiState) setCFARegister(id RegisterId) error {
	top, err := state.top()
	if err != nil {
		return err
	}

	top.SetCFARegisterOffset(id, top.CanonicalFrameAddress.Offset)
	return nil
}

func (state *cfiState) setCFAOffset(offset int64) error {
	top, err := state.top()
	if err != nil {
		return err
	}

	top.SetCFARegisterOffset(top.CanonicalFrameAddress.RegisterId, offset)
	return nil
}

func (state *cfiState) advanceLoc(delta uint64) error {
	state.location += elf.FileAddress(delta * state.CodeAlignmentFactor)
	return nil
}

func (state *cfiState) saveCIERules() error {
	top, err := state.top()
	if err != nil {
		return err
	}

	state.cieRules = top.Copy()
	return nil
}

func (state *cfiState) setRegisterRule(
	id RegisterId,
	rule RegisterRule,
) error {
	top, err := state.top()
	if err != nil {
		return err
	}

	top.SetRegisterRule(id, rule)
	return nil
}

func (state *cfiState) restoreRegisterRule(regId RegisterId) error {
	if state.cieRules == nil {
		return fmt.Errorf("cie rules not available")
	}

	rule, err := state.cieRules.GetRegisterRule(regId)
	if err != nil {
		return err
	}

	return state.setRegisterRule(regId, rule)
}

func (state *cfiState) setOffsetRule(
	regId RegisterId,
	offset int64,
	isValueOffset bool,
) error {
	kind := OffsetRule
	if isValueOffset {
		kind = ValueOffsetRule
	}

	return state.setRegisterRule(
		regId,
		RegisterRule{
			Kind:   kind,
			Offset: offset,
		})
}

func (state *cfiState) executeInstruction(decode *framePointerDecoder) error {
	opCode, err := decode.U8()
	if err != nil {
		return fmt.Errorf("failed to decode op code: %w", err)
	}

	if opCode == 0 { // The instruction stream may include 0 for whatever reason
		return nil
	}

	primaryOpCode := opCode & 0xc0 // upper 2 bits
	opCodeArg := opCode & 0x3f     // lower 6 bits

	if primaryOpCode > 0 { // i.e., is valid primary op code
		switch primaryOpCode {
		case DW_CFA_advance_loc:
			return state.advanceLoc(uint64(opCodeArg))
		case DW_CFA_offset:
			value, err := decode.ULEB128(64)
			if err != nil {
				return fmt.Errorf("failed to decode offset: %w", err)
			}

			return state.setOffsetRule(
				RegisterId(opCodeArg),
				int64(value)*state.DataAlignmentFactor,
				false)
		case DW_CFA_restore:
			return state.restoreRegisterRule(RegisterId(opCodeArg))
		default:
			panic("should never reach here")
		}
	}

	// opCodeArg is an extended op code
	switch opCodeArg {
	case DW_CFA_set_loc:
		address, err := decode.framePointer(state.PointerEncoding)
		if err != nil {
			return fmt.Errorf("failed to decode location: %w", err)
		}

		state.location = address
		return nil
	case DW_CFA_advance_loc1:
		delta, err := decode.U8()
		if err != nil {
			return fmt.Errorf("failed to decode offset: %w", err)
		}

		return state.advanceLoc(uint64(delta))
	case DW_CFA_advance_loc2:
		delta, err := decode.U16()
		if err != nil {
			return fmt.Errorf("failed to decode offset: %w", err)
		}

		return state.advanceLoc(uint64(delta))
	case DW_CFA_advance_loc4:
		delta, err := decode.U32()
		if err != nil {
			return fmt.Errorf("failed to decode offset: %w", err)
		}

		return state.advanceLoc(uint64(delta))
	case DW_CFA_def_cfa:
		id, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode register id: %w", err)
		}

		offset, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode offset: %w", err)
		}

		return state.setCFARegisterOffset(RegisterId(id), int64(offset))
	case DW_CFA_def_cfa_sf:
		id, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode register id: %w", err)
		}

		offset, err := decode.SLEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode offset: %w", err)
		}

		return state.setCFARegisterOffset(
			RegisterId(id),
			offset*state.DataAlignmentFactor)
	case DW_CFA_def_cfa_register:
		id, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode register id: %w", err)
		}

		return state.setCFARegister(RegisterId(id))
	case DW_CFA_def_cfa_offset:
		offset, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode register id: %w", err)
		}

		return state.setCFAOffset(int64(offset))
	case DW_CFA_def_cfa_offset_sf:
		offset, err := decode.SLEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode register id: %w", err)
		}

		return state.setCFAOffset(offset * state.DataAlignmentFactor)
	case DW_CFA_def_cfa_expression:
		return fmt.Errorf("dwarf expression not implemented")
	case DW_CFA_undefined:
		id, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode register id: %w", err)
		}

		return state.setRegisterRule(
			RegisterId(id),
			RegisterRule{
				Kind: UndefinedRule,
			})
	case DW_CFA_same_value:
		id, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode register id: %w", err)
		}

		return state.setRegisterRule(
			RegisterId(id),
			RegisterRule{
				Kind: SameValueRule,
			})
	case DW_CFA_offset_extended, DW_CFA_val_offset:
		id, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode register id: %w", err)
		}

		value, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode offset: %w", err)
		}

		return state.setOffsetRule(
			RegisterId(id),
			int64(value)*state.DataAlignmentFactor,
			opCodeArg == DW_CFA_val_offset)
	case DW_CFA_offset_extended_sf, DW_CFA_val_offset_sf:
		id, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode register id: %w", err)
		}

		value, err := decode.SLEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode offset: %w", err)
		}

		return state.setOffsetRule(
			RegisterId(id),
			value*state.DataAlignmentFactor,
			opCodeArg == DW_CFA_val_offset_sf)
	case DW_CFA_register:
		id, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode register id: %w", err)
		}

		other, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode other register id: %w", err)
		}

		return state.setRegisterRule(
			RegisterId(id),
			RegisterRule{
				Kind:       InRegisterRule,
				RegisterId: RegisterId(other),
			})
	case DW_CFA_expression:
		return fmt.Errorf("dwarf expression not implemented")
	case DW_CFA_val_expression:
		return fmt.Errorf("dwarf expression not implemented")
	case DW_CFA_restore_extended:
		id, err := decode.ULEB128(64)
		if err != nil {
			return fmt.Errorf("failed to decode register id: %w", err)
		}

		return state.restoreRegisterRule(RegisterId(id))
	case DW_CFA_remember_state:
		return state.push()
	case DW_CFA_restore_state:
		return state.pop()
	}

	return fmt.Errorf("unknown op code %d", opCode)
}
