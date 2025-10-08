// NOTE: This is based on based on dwarf.h from github.com/TartanLlama/sdb

package dwarf

import (
	"fmt"
)

// See dwarf 5 tablel 7.9 for full list
type Operation uint64

const (
	DW_OP_addr                = Operation(0x03)
	DW_OP_deref               = Operation(0x06)
	DW_OP_const1u             = Operation(0x08)
	DW_OP_const1s             = Operation(0x09)
	DW_OP_const2u             = Operation(0x0a)
	DW_OP_const2s             = Operation(0x0b)
	DW_OP_const4u             = Operation(0x0c)
	DW_OP_const4s             = Operation(0x0d)
	DW_OP_const8u             = Operation(0x0e)
	DW_OP_const8s             = Operation(0x0f)
	DW_OP_constu              = Operation(0x10)
	DW_OP_consts              = Operation(0x11)
	DW_OP_dup                 = Operation(0x12)
	DW_OP_drop                = Operation(0x13)
	DW_OP_over                = Operation(0x14)
	DW_OP_pick                = Operation(0x15)
	DW_OP_swap                = Operation(0x16)
	DW_OP_rot                 = Operation(0x17)
	DW_OP_xderef              = Operation(0x18)
	DW_OP_abs                 = Operation(0x19)
	DW_OP_and                 = Operation(0x1a)
	DW_OP_div                 = Operation(0x1b)
	DW_OP_minus               = Operation(0x1c)
	DW_OP_mod                 = Operation(0x1d)
	DW_OP_mul                 = Operation(0x1e)
	DW_OP_neg                 = Operation(0x1f)
	DW_OP_not                 = Operation(0x20)
	DW_OP_or                  = Operation(0x21)
	DW_OP_plus                = Operation(0x22)
	DW_OP_plus_uconst         = Operation(0x23)
	DW_OP_shl                 = Operation(0x24)
	DW_OP_shr                 = Operation(0x25)
	DW_OP_shra                = Operation(0x26)
	DW_OP_xor                 = Operation(0x27)
	DW_OP_skip                = Operation(0x2f)
	DW_OP_bra                 = Operation(0x28)
	DW_OP_eq                  = Operation(0x29)
	DW_OP_ge                  = Operation(0x2a)
	DW_OP_gt                  = Operation(0x2b)
	DW_OP_le                  = Operation(0x2c)
	DW_OP_lt                  = Operation(0x2d)
	DW_OP_ne                  = Operation(0x2e)
	DW_OP_lit0                = Operation(0x30)
	DW_OP_lit1                = Operation(0x31)
	DW_OP_lit2                = Operation(0x32)
	DW_OP_lit3                = Operation(0x33)
	DW_OP_lit4                = Operation(0x34)
	DW_OP_lit5                = Operation(0x35)
	DW_OP_lit6                = Operation(0x36)
	DW_OP_lit7                = Operation(0x37)
	DW_OP_lit8                = Operation(0x38)
	DW_OP_lit9                = Operation(0x39)
	DW_OP_lit10               = Operation(0x3a)
	DW_OP_lit11               = Operation(0x3b)
	DW_OP_lit12               = Operation(0x3c)
	DW_OP_lit13               = Operation(0x3d)
	DW_OP_lit14               = Operation(0x3e)
	DW_OP_lit15               = Operation(0x3f)
	DW_OP_lit16               = Operation(0x40)
	DW_OP_lit17               = Operation(0x41)
	DW_OP_lit18               = Operation(0x42)
	DW_OP_lit19               = Operation(0x43)
	DW_OP_lit20               = Operation(0x44)
	DW_OP_lit21               = Operation(0x45)
	DW_OP_lit22               = Operation(0x46)
	DW_OP_lit23               = Operation(0x47)
	DW_OP_lit24               = Operation(0x48)
	DW_OP_lit25               = Operation(0x49)
	DW_OP_lit26               = Operation(0x4a)
	DW_OP_lit27               = Operation(0x4b)
	DW_OP_lit28               = Operation(0x4c)
	DW_OP_lit29               = Operation(0x4d)
	DW_OP_lit30               = Operation(0x4e)
	DW_OP_lit31               = Operation(0x4f)
	DW_OP_reg0                = Operation(0x50)
	DW_OP_reg1                = Operation(0x51)
	DW_OP_reg2                = Operation(0x52)
	DW_OP_reg3                = Operation(0x53)
	DW_OP_reg4                = Operation(0x54)
	DW_OP_reg5                = Operation(0x55)
	DW_OP_reg6                = Operation(0x56)
	DW_OP_reg7                = Operation(0x57)
	DW_OP_reg8                = Operation(0x58)
	DW_OP_reg9                = Operation(0x59)
	DW_OP_reg10               = Operation(0x5a)
	DW_OP_reg11               = Operation(0x5b)
	DW_OP_reg12               = Operation(0x5c)
	DW_OP_reg13               = Operation(0x5d)
	DW_OP_reg14               = Operation(0x5e)
	DW_OP_reg15               = Operation(0x5f)
	DW_OP_reg16               = Operation(0x60)
	DW_OP_reg17               = Operation(0x61)
	DW_OP_reg18               = Operation(0x62)
	DW_OP_reg19               = Operation(0x63)
	DW_OP_reg20               = Operation(0x64)
	DW_OP_reg21               = Operation(0x65)
	DW_OP_reg22               = Operation(0x66)
	DW_OP_reg23               = Operation(0x67)
	DW_OP_reg24               = Operation(0x68)
	DW_OP_reg25               = Operation(0x69)
	DW_OP_reg26               = Operation(0x6a)
	DW_OP_reg27               = Operation(0x6b)
	DW_OP_reg28               = Operation(0x6c)
	DW_OP_reg29               = Operation(0x6d)
	DW_OP_reg30               = Operation(0x6e)
	DW_OP_reg31               = Operation(0x6f)
	DW_OP_breg0               = Operation(0x70)
	DW_OP_breg1               = Operation(0x71)
	DW_OP_breg2               = Operation(0x72)
	DW_OP_breg3               = Operation(0x73)
	DW_OP_breg4               = Operation(0x74)
	DW_OP_breg5               = Operation(0x75)
	DW_OP_breg6               = Operation(0x76)
	DW_OP_breg7               = Operation(0x77)
	DW_OP_breg8               = Operation(0x78)
	DW_OP_breg9               = Operation(0x79)
	DW_OP_breg10              = Operation(0x7a)
	DW_OP_breg11              = Operation(0x7b)
	DW_OP_breg12              = Operation(0x7c)
	DW_OP_breg13              = Operation(0x7d)
	DW_OP_breg14              = Operation(0x7e)
	DW_OP_breg15              = Operation(0x7f)
	DW_OP_breg16              = Operation(0x80)
	DW_OP_breg17              = Operation(0x81)
	DW_OP_breg18              = Operation(0x82)
	DW_OP_breg19              = Operation(0x83)
	DW_OP_breg20              = Operation(0x84)
	DW_OP_breg21              = Operation(0x85)
	DW_OP_breg22              = Operation(0x86)
	DW_OP_breg23              = Operation(0x87)
	DW_OP_breg24              = Operation(0x88)
	DW_OP_breg25              = Operation(0x89)
	DW_OP_breg26              = Operation(0x8a)
	DW_OP_breg27              = Operation(0x8b)
	DW_OP_breg28              = Operation(0x8c)
	DW_OP_breg29              = Operation(0x8d)
	DW_OP_breg30              = Operation(0x8e)
	DW_OP_breg31              = Operation(0x8f)
	DW_OP_regx                = Operation(0x90)
	DW_OP_fbreg               = Operation(0x91)
	DW_OP_bregx               = Operation(0x92)
	DW_OP_piece               = Operation(0x93)
	DW_OP_deref_size          = Operation(0x94)
	DW_OP_xderef_size         = Operation(0x95)
	DW_OP_nop                 = Operation(0x96)
	DW_OP_push_object_address = Operation(0x97)
	DW_OP_call2               = Operation(0x98)
	DW_OP_call4               = Operation(0x99)
	DW_OP_call_ref            = Operation(0x9a)
	DW_OP_form_tls_address    = Operation(0x9b)
	DW_OP_call_frame_cfa      = Operation(0x9c)
	DW_OP_bit_piece           = Operation(0x9d)
	DW_OP_implicit_value      = Operation(0x9e)
	DW_OP_stack_value         = Operation(0x9f)
	DW_OP_lo_user             = Operation(0xe0)
	DW_OP_hi_user             = Operation(0xff)
)

func (operation Operation) String() string {
	switch operation {
	case DW_OP_addr:
		return "DW_OP_addr"
	case DW_OP_deref:
		return "DW_OP_deref"
	case DW_OP_const1u:
		return "DW_OP_const1u"
	case DW_OP_const1s:
		return "DW_OP_const1s"
	case DW_OP_const2u:
		return "DW_OP_const2u"
	case DW_OP_const2s:
		return "DW_OP_const2s"
	case DW_OP_const4u:
		return "DW_OP_const4u"
	case DW_OP_const4s:
		return "DW_OP_const4s"
	case DW_OP_const8u:
		return "DW_OP_const8u"
	case DW_OP_const8s:
		return "DW_OP_const8s"
	case DW_OP_constu:
		return "DW_OP_constu"
	case DW_OP_consts:
		return "DW_OP_consts"
	case DW_OP_dup:
		return "DW_OP_dup"
	case DW_OP_drop:
		return "DW_OP_drop"
	case DW_OP_over:
		return "DW_OP_over"
	case DW_OP_pick:
		return "DW_OP_pick"
	case DW_OP_swap:
		return "DW_OP_swap"
	case DW_OP_rot:
		return "DW_OP_rot"
	case DW_OP_xderef:
		return "DW_OP_xderef"
	case DW_OP_abs:
		return "DW_OP_abs"
	case DW_OP_and:
		return "DW_OP_and"
	case DW_OP_div:
		return "DW_OP_div"
	case DW_OP_minus:
		return "DW_OP_minus"
	case DW_OP_mod:
		return "DW_OP_mod"
	case DW_OP_mul:
		return "DW_OP_mul"
	case DW_OP_neg:
		return "DW_OP_neg"
	case DW_OP_not:
		return "DW_OP_not"
	case DW_OP_or:
		return "DW_OP_or"
	case DW_OP_plus:
		return "DW_OP_plus"
	case DW_OP_plus_uconst:
		return "DW_OP_plus_uconst"
	case DW_OP_shl:
		return "DW_OP_shl"
	case DW_OP_shr:
		return "DW_OP_shr"
	case DW_OP_shra:
		return "DW_OP_shra"
	case DW_OP_xor:
		return "DW_OP_xor"
	case DW_OP_skip:
		return "DW_OP_skip"
	case DW_OP_bra:
		return "DW_OP_bra"
	case DW_OP_eq:
		return "DW_OP_eq"
	case DW_OP_ge:
		return "DW_OP_ge"
	case DW_OP_gt:
		return "DW_OP_gt"
	case DW_OP_le:
		return "DW_OP_le"
	case DW_OP_lt:
		return "DW_OP_lt"
	case DW_OP_ne:
		return "DW_OP_ne"
	case DW_OP_lit0:
		return "DW_OP_lit0"
	case DW_OP_lit1:
		return "DW_OP_lit1"
	case DW_OP_lit2:
		return "DW_OP_lit2"
	case DW_OP_lit3:
		return "DW_OP_lit3"
	case DW_OP_lit4:
		return "DW_OP_lit4"
	case DW_OP_lit5:
		return "DW_OP_lit5"
	case DW_OP_lit6:
		return "DW_OP_lit6"
	case DW_OP_lit7:
		return "DW_OP_lit7"
	case DW_OP_lit8:
		return "DW_OP_lit8"
	case DW_OP_lit9:
		return "DW_OP_lit9"
	case DW_OP_lit10:
		return "DW_OP_lit10"
	case DW_OP_lit11:
		return "DW_OP_lit11"
	case DW_OP_lit12:
		return "DW_OP_lit12"
	case DW_OP_lit13:
		return "DW_OP_lit13"
	case DW_OP_lit14:
		return "DW_OP_lit14"
	case DW_OP_lit15:
		return "DW_OP_lit15"
	case DW_OP_lit16:
		return "DW_OP_lit16"
	case DW_OP_lit17:
		return "DW_OP_lit17"
	case DW_OP_lit18:
		return "DW_OP_lit18"
	case DW_OP_lit19:
		return "DW_OP_lit19"
	case DW_OP_lit20:
		return "DW_OP_lit20"
	case DW_OP_lit21:
		return "DW_OP_lit21"
	case DW_OP_lit22:
		return "DW_OP_lit22"
	case DW_OP_lit23:
		return "DW_OP_lit23"
	case DW_OP_lit24:
		return "DW_OP_lit24"
	case DW_OP_lit25:
		return "DW_OP_lit25"
	case DW_OP_lit26:
		return "DW_OP_lit26"
	case DW_OP_lit27:
		return "DW_OP_lit27"
	case DW_OP_lit28:
		return "DW_OP_lit28"
	case DW_OP_lit29:
		return "DW_OP_lit29"
	case DW_OP_lit30:
		return "DW_OP_lit30"
	case DW_OP_lit31:
		return "DW_OP_lit31"
	case DW_OP_reg0:
		return "DW_OP_reg0"
	case DW_OP_reg1:
		return "DW_OP_reg1"
	case DW_OP_reg2:
		return "DW_OP_reg2"
	case DW_OP_reg3:
		return "DW_OP_reg3"
	case DW_OP_reg4:
		return "DW_OP_reg4"
	case DW_OP_reg5:
		return "DW_OP_reg5"
	case DW_OP_reg6:
		return "DW_OP_reg6"
	case DW_OP_reg7:
		return "DW_OP_reg7"
	case DW_OP_reg8:
		return "DW_OP_reg8"
	case DW_OP_reg9:
		return "DW_OP_reg9"
	case DW_OP_reg10:
		return "DW_OP_reg10"
	case DW_OP_reg11:
		return "DW_OP_reg11"
	case DW_OP_reg12:
		return "DW_OP_reg12"
	case DW_OP_reg13:
		return "DW_OP_reg13"
	case DW_OP_reg14:
		return "DW_OP_reg14"
	case DW_OP_reg15:
		return "DW_OP_reg15"
	case DW_OP_reg16:
		return "DW_OP_reg16"
	case DW_OP_reg17:
		return "DW_OP_reg17"
	case DW_OP_reg18:
		return "DW_OP_reg18"
	case DW_OP_reg19:
		return "DW_OP_reg19"
	case DW_OP_reg20:
		return "DW_OP_reg20"
	case DW_OP_reg21:
		return "DW_OP_reg21"
	case DW_OP_reg22:
		return "DW_OP_reg22"
	case DW_OP_reg23:
		return "DW_OP_reg23"
	case DW_OP_reg24:
		return "DW_OP_reg24"
	case DW_OP_reg25:
		return "DW_OP_reg25"
	case DW_OP_reg26:
		return "DW_OP_reg26"
	case DW_OP_reg27:
		return "DW_OP_reg27"
	case DW_OP_reg28:
		return "DW_OP_reg28"
	case DW_OP_reg29:
		return "DW_OP_reg29"
	case DW_OP_reg30:
		return "DW_OP_reg30"
	case DW_OP_reg31:
		return "DW_OP_reg31"
	case DW_OP_breg0:
		return "DW_OP_breg0"
	case DW_OP_breg1:
		return "DW_OP_breg1"
	case DW_OP_breg2:
		return "DW_OP_breg2"
	case DW_OP_breg3:
		return "DW_OP_breg3"
	case DW_OP_breg4:
		return "DW_OP_breg4"
	case DW_OP_breg5:
		return "DW_OP_breg5"
	case DW_OP_breg6:
		return "DW_OP_breg6"
	case DW_OP_breg7:
		return "DW_OP_breg7"
	case DW_OP_breg8:
		return "DW_OP_breg8"
	case DW_OP_breg9:
		return "DW_OP_breg9"
	case DW_OP_breg10:
		return "DW_OP_breg10"
	case DW_OP_breg11:
		return "DW_OP_breg11"
	case DW_OP_breg12:
		return "DW_OP_breg12"
	case DW_OP_breg13:
		return "DW_OP_breg13"
	case DW_OP_breg14:
		return "DW_OP_breg14"
	case DW_OP_breg15:
		return "DW_OP_breg15"
	case DW_OP_breg16:
		return "DW_OP_breg16"
	case DW_OP_breg17:
		return "DW_OP_breg17"
	case DW_OP_breg18:
		return "DW_OP_breg18"
	case DW_OP_breg19:
		return "DW_OP_breg19"
	case DW_OP_breg20:
		return "DW_OP_breg20"
	case DW_OP_breg21:
		return "DW_OP_breg21"
	case DW_OP_breg22:
		return "DW_OP_breg22"
	case DW_OP_breg23:
		return "DW_OP_breg23"
	case DW_OP_breg24:
		return "DW_OP_breg24"
	case DW_OP_breg25:
		return "DW_OP_breg25"
	case DW_OP_breg26:
		return "DW_OP_breg26"
	case DW_OP_breg27:
		return "DW_OP_breg27"
	case DW_OP_breg28:
		return "DW_OP_breg28"
	case DW_OP_breg29:
		return "DW_OP_breg29"
	case DW_OP_breg30:
		return "DW_OP_breg30"
	case DW_OP_breg31:
		return "DW_OP_breg31"
	case DW_OP_regx:
		return "DW_OP_regx"
	case DW_OP_fbreg:
		return "DW_OP_fbreg"
	case DW_OP_bregx:
		return "DW_OP_bregx"
	case DW_OP_piece:
		return "DW_OP_piece"
	case DW_OP_deref_size:
		return "DW_OP_deref_size"
	case DW_OP_xderef_size:
		return "DW_OP_xderef_size"
	case DW_OP_nop:
		return "DW_OP_nop"
	case DW_OP_push_object_address:
		return "DW_OP_push_object_address"
	case DW_OP_call2:
		return "DW_OP_call2"
	case DW_OP_call4:
		return "DW_OP_call4"
	case DW_OP_call_ref:
		return "DW_OP_call_ref"
	case DW_OP_form_tls_address:
		return "DW_OP_form_tls_address"
	case DW_OP_call_frame_cfa:
		return "DW_OP_call_frame_cfa"
	case DW_OP_bit_piece:
		return "DW_OP_bit_piece"
	case DW_OP_implicit_value:
		return "DW_OP_implicit_value"
	case DW_OP_stack_value:
		return "DW_OP_stack_value"
	case DW_OP_lo_user:
		return "DW_OP_lo_user"
	case DW_OP_hi_user:
		return "DW_OP_hi_user"
	default:
		return fmt.Sprintf("DW_OP_unknown_%d", operation)
	}
}
