// NOTE: This is based on based on dwarf.h from github.com/TartanLlama/sdb

package dwarf

import (
	"fmt"
)

// See dwarf 5 table 7.6 for full list
type Format uint64

const (
	DW_FORM_addr         = Format(0x01)
	DW_FORM_block2       = Format(0x03)
	DW_FORM_block4       = Format(0x04)
	DW_FORM_data2        = Format(0x05)
	DW_FORM_data4        = Format(0x06)
	DW_FORM_data8        = Format(0x07)
	DW_FORM_string       = Format(0x08)
	DW_FORM_block        = Format(0x09)
	DW_FORM_block1       = Format(0x0a)
	DW_FORM_data1        = Format(0x0b)
	DW_FORM_flag         = Format(0x0c)
	DW_FORM_sdata        = Format(0x0d)
	DW_FORM_strp         = Format(0x0e)
	DW_FORM_udata        = Format(0x0f)
	DW_FORM_ref_addr     = Format(0x10)
	DW_FORM_ref1         = Format(0x11)
	DW_FORM_ref2         = Format(0x12)
	DW_FORM_ref4         = Format(0x13)
	DW_FORM_ref8         = Format(0x14)
	DW_FORM_ref_udata    = Format(0x15)
	DW_FORM_indirect     = Format(0x16)
	DW_FORM_sec_offset   = Format(0x17)
	DW_FORM_exprloc      = Format(0x18)
	DW_FORM_flag_present = Format(0x19)
	DW_FORM_ref_sig8     = Format(0x20)
)

func (format Format) String() string {
	switch format {
	case DW_FORM_addr:
		return "DW_FORM_addr"
	case DW_FORM_block2:
		return "DW_FORM_block2"
	case DW_FORM_block4:
		return "DW_FORM_block4"
	case DW_FORM_data2:
		return "DW_FORM_data2"
	case DW_FORM_data4:
		return "DW_FORM_data4"
	case DW_FORM_data8:
		return "DW_FORM_data8"
	case DW_FORM_string:
		return "DW_FORM_string"
	case DW_FORM_block:
		return "DW_FORM_block"
	case DW_FORM_block1:
		return "DW_FORM_block1"
	case DW_FORM_data1:
		return "DW_FORM_data1"
	case DW_FORM_flag:
		return "DW_FORM_flag"
	case DW_FORM_sdata:
		return "DW_FORM_sdata"
	case DW_FORM_strp:
		return "DW_FORM_strp"
	case DW_FORM_udata:
		return "DW_FORM_udata"
	case DW_FORM_ref_addr:
		return "DW_FORM_ref_addr"
	case DW_FORM_ref1:
		return "DW_FORM_ref1"
	case DW_FORM_ref2:
		return "DW_FORM_ref2"
	case DW_FORM_ref4:
		return "DW_FORM_ref4"
	case DW_FORM_ref8:
		return "DW_FORM_ref8"
	case DW_FORM_ref_udata:
		return "DW_FORM_ref_udata"
	case DW_FORM_indirect:
		return "DW_FORM_indirect"
	case DW_FORM_sec_offset:
		return "DW_FORM_sec_offset"
	case DW_FORM_exprloc:
		return "DW_FORM_exprloc"
	case DW_FORM_flag_present:
		return "DW_FORM_flag_present"
	case DW_FORM_ref_sig8:
		return "DW_FORM_ref_sig8"
	default:
		return fmt.Sprintf("DW_FORM_unknown_%d", format)
	}
}
