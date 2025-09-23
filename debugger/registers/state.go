package registers

import (
	"fmt"
	"reflect"

	"github.com/pattyshack/bad/ptrace"
)

type State struct {
	gpr ptrace.UserRegs
	fpr ptrace.UserFPRegs
	dr  [8]uintptr
}

// This always returns Uint8 / Uint16 / Uint32 / Uint64 / Uint128 depending on
// the register size.
func (state State) Value(reg Spec) Value {
	var data reflect.Value
	switch reg.Class {
	case GeneralClass:
		data = reflect.ValueOf(state.gpr)
	case FloatingPointClass:
		if reg.Field == stSpace {
			return U128(
				state.fpr.StSpace[2*reg.Index+1],
				state.fpr.StSpace[2*reg.Index])
		}

		if reg.Field == xmmSpace {
			return U128(
				state.fpr.XmmSpace[2*reg.Index+1],
				state.fpr.XmmSpace[2*reg.Index])
		}

		data = reflect.ValueOf(state.fpr)
	case DebugClass:
		return U64(uint64(state.dr[reg.Index]))
	default:
		panic(fmt.Sprintf("invalid register: %#v", reg))
	}

	value := data.FieldByName(reg.Field).Uint()
	switch reg.Size {
	case 1:
		if reg.IsHighRegister {
			value >>= 8
		}

		return U8(uint8(value))
	case 2:
		return U16(uint16(value))
	case 4:
		return U32(uint32(value))
	case 8:
		return U64(value)
	default:
		panic(fmt.Sprintf("invalid register: %#v", reg))
	}
}

func (state State) WithValue(
	reg Spec,
	value Value,
) (
	State,
	error,
) {
	err := reg.CanAccept(value)
	if err != nil {
		return State{}, err
	}

	newState := state

	var data reflect.Value
	switch reg.Class {
	case GeneralClass:
		data = reflect.Indirect(reflect.ValueOf(&newState.gpr))
	case FloatingPointClass:
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
	case DebugClass:
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
