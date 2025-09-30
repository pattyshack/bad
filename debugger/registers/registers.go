package registers

import (
	"fmt"
	"reflect"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/ptrace"
)

var (
	userDebugRegistersOffset = uintptr(0) // initialized by init()
)

type Registers struct {
	tracer *ptrace.Tracer
}

func New(tracer *ptrace.Tracer) *Registers {
	return &Registers{
		tracer: tracer,
	}
}

func (registers *Registers) GetState() (State, error) {
	gpr, err := registers.tracer.GetGeneralRegisters()
	if err != nil {
		return State{}, err
	}

	fpr, err := registers.tracer.GetFloatingPointRegisters()
	if err != nil {
		return State{}, err
	}

	state := State{
		gpr: *gpr,
		fpr: *fpr,
	}

	for idx, _ := range state.dr {
		offset := userDebugRegistersOffset + uintptr(idx*8)
		value, err := registers.tracer.PeekUserArea(offset)
		if err != nil {
			return State{}, err
		}
		state.dr[idx] = value
	}

	return state, nil
}

func (registers *Registers) SetState(state State) error {
	if len(state.undefined) > 0 {
		return fmt.Errorf("cannot set register state with undefined values")
	}

	err := registers.tracer.SetGeneralRegisters(&state.gpr)
	if err != nil {
		return err
	}

	err = registers.tracer.SetFloatingPointRegisters(&state.fpr)
	if err != nil {
		return err
	}

	for idx, value := range state.dr {
		// dr4 and dr5 are not real registers
		// https://en.wikipedia.org/wiki/X86_debug_register
		if idx == 4 || idx == 5 {
			continue
		}

		offset := userDebugRegistersOffset + uintptr(idx*8)
		err := registers.tracer.PokeUserArea(offset, value)
		if err != nil {
			return fmt.Errorf("failed to set dr%d: %w", idx, err)
		}
	}

	return nil
}

func (registers *Registers) GetProgramCounter() (State, VirtualAddress, error) {
	state, err := registers.GetState()
	if err != nil {
		return State{}, 0, fmt.Errorf("failed to read program counter: %w", err)
	}

	return state, VirtualAddress(state.Value(ProgramCounter).ToUint64()), nil
}

func (registers *Registers) SetProgramCounter(address VirtualAddress) error {
	state, err := registers.GetState()
	if err != nil {
		return fmt.Errorf("failed to read program counter: %w", err)
	}

	newState, err := state.WithValue(ProgramCounter, U64(uint64(address)))
	if err != nil {
		return fmt.Errorf(
			"failed to update program counter state to %s: %w",
			address,
			err)
	}

	err = registers.SetState(newState)
	if err != nil {
		return fmt.Errorf("failed to set program counter to %s: %w", address, err)
	}

	return nil
}

func init() {
	user := ptrace.User{}
	userType := reflect.TypeOf(user)

	field, ok := userType.FieldByName(uDebugReg)
	if !ok {
		panic("should never happen")
	}
	userDebugRegistersOffset = field.Offset
}
