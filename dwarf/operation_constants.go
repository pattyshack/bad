// NOTE: This is based on based on dwarf.h from github.com/TartanLlama/sdb

package dwarf

import (
	"fmt"
)

// See dwarf 5 tablel 7.9 for full list
type Operation uint64

const (
	DW_OP_addr                = 0x03
	DW_OP_deref               = 0x06
	DW_OP_const1u             = 0x08
	DW_OP_const1s             = 0x09
	DW_OP_const2u             = 0x0a
	DW_OP_const2s             = 0x0b
	DW_OP_const4u             = 0x0c
	DW_OP_const4s             = 0x0d
	DW_OP_const8u             = 0x0e
	DW_OP_const8s             = 0x0f
	DW_OP_constu              = 0x10
	DW_OP_consts              = 0x11
	DW_OP_dup                 = 0x12
	DW_OP_drop                = 0x13
	DW_OP_over                = 0x14
	DW_OP_pick                = 0x15
	DW_OP_swap                = 0x16
	DW_OP_rot                 = 0x17
	DW_OP_xderef              = 0x18
	DW_OP_abs                 = 0x19
	DW_OP_and                 = 0x1a
	DW_OP_div                 = 0x1b
	DW_OP_minus               = 0x1c
	DW_OP_mod                 = 0x1d
	DW_OP_mul                 = 0x1e
	DW_OP_neg                 = 0x1f
	DW_OP_not                 = 0x20
	DW_OP_or                  = 0x21
	DW_OP_plus                = 0x22
	DW_OP_plus_uconst         = 0x23
	DW_OP_shl                 = 0x24
	DW_OP_shr                 = 0x25
	DW_OP_shra                = 0x26
	DW_OP_xor                 = 0x27
	DW_OP_skip                = 0x2f
	DW_OP_bra                 = 0x28
	DW_OP_eq                  = 0x29
	DW_OP_ge                  = 0x2a
	DW_OP_gt                  = 0x2b
	DW_OP_le                  = 0x2c
	DW_OP_lt                  = 0x2d
	DW_OP_ne                  = 0x2e
	DW_OP_lit0                = 0x30
	DW_OP_lit1                = 0x31
	DW_OP_lit2                = 0x32
	DW_OP_lit3                = 0x33
	DW_OP_lit4                = 0x34
	DW_OP_lit5                = 0x35
	DW_OP_lit6                = 0x36
	DW_OP_lit7                = 0x37
	DW_OP_lit8                = 0x38
	DW_OP_lit9                = 0x39
	DW_OP_lit10               = 0x3a
	DW_OP_lit11               = 0x3b
	DW_OP_lit12               = 0x3c
	DW_OP_lit13               = 0x3d
	DW_OP_lit14               = 0x3e
	DW_OP_lit15               = 0x3f
	DW_OP_lit16               = 0x40
	DW_OP_lit17               = 0x41
	DW_OP_lit18               = 0x42
	DW_OP_lit19               = 0x43
	DW_OP_lit20               = 0x44
	DW_OP_lit21               = 0x45
	DW_OP_lit22               = 0x46
	DW_OP_lit23               = 0x47
	DW_OP_lit24               = 0x48
	DW_OP_lit25               = 0x49
	DW_OP_lit26               = 0x4a
	DW_OP_lit27               = 0x4b
	DW_OP_lit28               = 0x4c
	DW_OP_lit29               = 0x4d
	DW_OP_lit30               = 0x4e
	DW_OP_lit31               = 0x4f
	DW_OP_reg0                = 0x50
	DW_OP_reg1                = 0x51
	DW_OP_reg2                = 0x52
	DW_OP_reg3                = 0x53
	DW_OP_reg4                = 0x54
	DW_OP_reg5                = 0x55
	DW_OP_reg6                = 0x56
	DW_OP_reg7                = 0x57
	DW_OP_reg8                = 0x58
	DW_OP_reg9                = 0x59
	DW_OP_reg10               = 0x5a
	DW_OP_reg11               = 0x5b
	DW_OP_reg12               = 0x5c
	DW_OP_reg13               = 0x5d
	DW_OP_reg14               = 0x5e
	DW_OP_reg15               = 0x5f
	DW_OP_reg16               = 0x60
	DW_OP_reg17               = 0x61
	DW_OP_reg18               = 0x62
	DW_OP_reg19               = 0x63
	DW_OP_reg20               = 0x64
	DW_OP_reg21               = 0x65
	DW_OP_reg22               = 0x66
	DW_OP_reg23               = 0x67
	DW_OP_reg24               = 0x68
	DW_OP_reg25               = 0x69
	DW_OP_reg26               = 0x6a
	DW_OP_reg27               = 0x6b
	DW_OP_reg28               = 0x6c
	DW_OP_reg29               = 0x6d
	DW_OP_reg30               = 0x6e
	DW_OP_reg31               = 0x6f
	DW_OP_breg0               = 0x70
	DW_OP_breg1               = 0x71
	DW_OP_breg2               = 0x72
	DW_OP_breg3               = 0x73
	DW_OP_breg4               = 0x74
	DW_OP_breg5               = 0x75
	DW_OP_breg6               = 0x76
	DW_OP_breg7               = 0x77
	DW_OP_breg8               = 0x78
	DW_OP_breg9               = 0x79
	DW_OP_breg10              = 0x7a
	DW_OP_breg11              = 0x7b
	DW_OP_breg12              = 0x7c
	DW_OP_breg13              = 0x7d
	DW_OP_breg14              = 0x7e
	DW_OP_breg15              = 0x7f
	DW_OP_breg16              = 0x80
	DW_OP_breg17              = 0x81
	DW_OP_breg18              = 0x82
	DW_OP_breg19              = 0x83
	DW_OP_breg20              = 0x84
	DW_OP_breg21              = 0x85
	DW_OP_breg22              = 0x86
	DW_OP_breg23              = 0x87
	DW_OP_breg24              = 0x88
	DW_OP_breg25              = 0x89
	DW_OP_breg26              = 0x8a
	DW_OP_breg27              = 0x8b
	DW_OP_breg28              = 0x8c
	DW_OP_breg29              = 0x8d
	DW_OP_breg30              = 0x8e
	DW_OP_breg31              = 0x8f
	DW_OP_regx                = 0x90
	DW_OP_fbreg               = 0x91
	DW_OP_bregx               = 0x92
	DW_OP_piece               = 0x93
	DW_OP_deref_size          = 0x94
	DW_OP_xderef_size         = 0x95
	DW_OP_nop                 = 0x96
	DW_OP_push_object_address = 0x97
	DW_OP_call2               = 0x98
	DW_OP_call4               = 0x99
	DW_OP_call_ref            = 0x9a
	DW_OP_form_tls_address    = 0x9b
	DW_OP_call_frame_cfa      = 0x9c
	DW_OP_bit_piece           = 0x9d
	DW_OP_implicit_value      = 0x9e
	DW_OP_stack_value         = 0x9f
	DW_OP_lo_user             = 0xe0
	DW_OP_hi_user             = 0xff
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
