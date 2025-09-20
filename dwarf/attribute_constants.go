// NOTE: This is based on based on dwarf.h from github.com/TartanLlama/sdb

package dwarf

import (
	"fmt"
)

// See dwarf 5 table 7.5 for full list
type Attribute uint64

const (
	DW_AT_sibling              = Attribute(0x01)
	DW_AT_location             = Attribute(0x02)
	DW_AT_name                 = Attribute(0x03)
	DW_AT_ordering             = Attribute(0x09)
	DW_AT_byte_size            = Attribute(0x0b)
	DW_AT_bit_offset           = Attribute(0x0c)
	DW_AT_bit_size             = Attribute(0x0d)
	DW_AT_stmt_list            = Attribute(0x10)
	DW_AT_low_pc               = Attribute(0x11)
	DW_AT_high_pc              = Attribute(0x12)
	DW_AT_language             = Attribute(0x13)
	DW_AT_discr                = Attribute(0x15)
	DW_AT_discr_value          = Attribute(0x16)
	DW_AT_visibility           = Attribute(0x17)
	DW_AT_import               = Attribute(0x18)
	DW_AT_string_length        = Attribute(0x19)
	DW_AT_common_reference     = Attribute(0x1a)
	DW_AT_comp_dir             = Attribute(0x1b)
	DW_AT_const_value          = Attribute(0x1c)
	DW_AT_containing_type      = Attribute(0x1d)
	DW_AT_default_value        = Attribute(0x1e)
	DW_AT_inline               = Attribute(0x20)
	DW_AT_is_optional          = Attribute(0x21)
	DW_AT_lower_bound          = Attribute(0x22)
	DW_AT_producer             = Attribute(0x25)
	DW_AT_prototyped           = Attribute(0x27)
	DW_AT_return_addr          = Attribute(0x2a)
	DW_AT_start_scope          = Attribute(0x2c)
	DW_AT_bit_stride           = Attribute(0x2e)
	DW_AT_upper_bound          = Attribute(0x2f)
	DW_AT_abstract_origin      = Attribute(0x31)
	DW_AT_accessibility        = Attribute(0x32)
	DW_AT_address_class        = Attribute(0x33)
	DW_AT_artificial           = Attribute(0x34)
	DW_AT_base_types           = Attribute(0x35)
	DW_AT_calling_convention   = Attribute(0x36)
	DW_AT_count                = Attribute(0x37)
	DW_AT_data_member_location = Attribute(0x38)
	DW_AT_decl_column          = Attribute(0x39)
	DW_AT_decl_file            = Attribute(0x3a)
	DW_AT_decl_line            = Attribute(0x3b)
	DW_AT_declaration          = Attribute(0x3c)
	DW_AT_discr_list           = Attribute(0x3d)
	DW_AT_encoding             = Attribute(0x3e)
	DW_AT_external             = Attribute(0x3f)
	DW_AT_frame_base           = Attribute(0x40)
	DW_AT_friend               = Attribute(0x41)
	DW_AT_identifier_case      = Attribute(0x42)
	DW_AT_macro_info           = Attribute(0x43)
	DW_AT_namelist_item        = Attribute(0x44)
	DW_AT_priority             = Attribute(0x45)
	DW_AT_segment              = Attribute(0x46)
	DW_AT_specification        = Attribute(0x47)
	DW_AT_static_link          = Attribute(0x48)
	DW_AT_type                 = Attribute(0x49)
	DW_AT_use_location         = Attribute(0x4a)
	DW_AT_variable_parameter   = Attribute(0x4b)
	DW_AT_virtuality           = Attribute(0x4c)
	DW_AT_vtable_elem_location = Attribute(0x4d)
	DW_AT_allocated            = Attribute(0x4e)
	DW_AT_associated           = Attribute(0x4f)
	DW_AT_data_location        = Attribute(0x50)
	DW_AT_byte_stride          = Attribute(0x51)
	DW_AT_entry_pc             = Attribute(0x52)
	DW_AT_use_UTF8             = Attribute(0x53)
	DW_AT_extension            = Attribute(0x54)
	DW_AT_ranges               = Attribute(0x55)
	DW_AT_trampoline           = Attribute(0x56)
	DW_AT_call_column          = Attribute(0x57)
	DW_AT_call_file            = Attribute(0x58)
	DW_AT_call_line            = Attribute(0x59)
	DW_AT_description          = Attribute(0x5a)
	DW_AT_binary_scale         = Attribute(0x5b)
	DW_AT_decimal_scale        = Attribute(0x5c)
	DW_AT_small                = Attribute(0x5d)
	DW_AT_decimal_sign         = Attribute(0x5e)
	DW_AT_digit_count          = Attribute(0x5f)
	DW_AT_picture_string       = Attribute(0x60)
	DW_AT_mutable              = Attribute(0x61)
	DW_AT_threads_scaled       = Attribute(0x62)
	DW_AT_explicit             = Attribute(0x63)
	DW_AT_object_pointer       = Attribute(0x64)
	DW_AT_endianity            = Attribute(0x65)
	DW_AT_elemental            = Attribute(0x66)
	DW_AT_pure                 = Attribute(0x67)
	DW_AT_recursive            = Attribute(0x68)
	DW_AT_signature            = Attribute(0x69)
	DW_AT_main_subprogram      = Attribute(0x6a)
	DW_AT_data_bit_offset      = Attribute(0x6b)
	DW_AT_const_expr           = Attribute(0x6c)
	DW_AT_enum_class           = Attribute(0x6d)
	DW_AT_linkage_name         = Attribute(0x6e)

	DW_AT_defaulted = Attribute(0x8b)

	DW_AT_lo_user = Attribute(0x2000)
	DW_AT_hi_user = Attribute(0x3fff)
)

func (attribute Attribute) String() string {
	switch attribute {
	case DW_AT_sibling:
		return "DW_AT_sibling"
	case DW_AT_location:
		return "DW_AT_location"
	case DW_AT_name:
		return "DW_AT_name"
	case DW_AT_ordering:
		return "DW_AT_ordering"
	case DW_AT_byte_size:
		return "DW_AT_byte_size"
	case DW_AT_bit_offset:
		return "DW_AT_bit_offset"
	case DW_AT_bit_size:
		return "DW_AT_bit_size"
	case DW_AT_stmt_list:
		return "DW_AT_stmt_list"
	case DW_AT_low_pc:
		return "DW_AT_low_pc"
	case DW_AT_high_pc:
		return "DW_AT_high_pc"
	case DW_AT_language:
		return "DW_AT_language"
	case DW_AT_discr:
		return "DW_AT_discr"
	case DW_AT_discr_value:
		return "DW_AT_discr_value"
	case DW_AT_visibility:
		return "DW_AT_visibility"
	case DW_AT_import:
		return "DW_AT_import"
	case DW_AT_string_length:
		return "DW_AT_string_length"
	case DW_AT_common_reference:
		return "DW_AT_common_reference"
	case DW_AT_comp_dir:
		return "DW_AT_comp_dir"
	case DW_AT_const_value:
		return "DW_AT_const_value"
	case DW_AT_containing_type:
		return "DW_AT_containing_type"
	case DW_AT_default_value:
		return "DW_AT_default_value"
	case DW_AT_inline:
		return "DW_AT_inline"
	case DW_AT_is_optional:
		return "DW_AT_is_optional"
	case DW_AT_lower_bound:
		return "DW_AT_lower_bound"
	case DW_AT_producer:
		return "DW_AT_producer"
	case DW_AT_prototyped:
		return "DW_AT_prototyped"
	case DW_AT_return_addr:
		return "DW_AT_return_addr"
	case DW_AT_start_scope:
		return "DW_AT_start_scope"
	case DW_AT_bit_stride:
		return "DW_AT_bit_stride"
	case DW_AT_upper_bound:
		return "DW_AT_upper_bound"
	case DW_AT_abstract_origin:
		return "DW_AT_abstract_origin"
	case DW_AT_accessibility:
		return "DW_AT_accessibility"
	case DW_AT_address_class:
		return "DW_AT_address_class"
	case DW_AT_artificial:
		return "DW_AT_artificial"
	case DW_AT_base_types:
		return "DW_AT_base_types"
	case DW_AT_calling_convention:
		return "DW_AT_calling_convention"
	case DW_AT_count:
		return "DW_AT_count"
	case DW_AT_data_member_location:
		return "DW_AT_data_member_location"
	case DW_AT_decl_column:
		return "DW_AT_decl_column"
	case DW_AT_decl_file:
		return "DW_AT_decl_file"
	case DW_AT_decl_line:
		return "DW_AT_decl_line"
	case DW_AT_declaration:
		return "DW_AT_declaration"
	case DW_AT_discr_list:
		return "DW_AT_discr_list"
	case DW_AT_encoding:
		return "DW_AT_encoding"
	case DW_AT_external:
		return "DW_AT_external"
	case DW_AT_frame_base:
		return "DW_AT_frame_base"
	case DW_AT_friend:
		return "DW_AT_friend"
	case DW_AT_identifier_case:
		return "DW_AT_identifier_case"
	case DW_AT_macro_info:
		return "DW_AT_macro_info"
	case DW_AT_namelist_item:
		return "DW_AT_namelist_item"
	case DW_AT_priority:
		return "DW_AT_priority"
	case DW_AT_segment:
		return "DW_AT_segment"
	case DW_AT_specification:
		return "DW_AT_specification"
	case DW_AT_static_link:
		return "DW_AT_static_link"
	case DW_AT_type:
		return "DW_AT_type"
	case DW_AT_use_location:
		return "DW_AT_use_location"
	case DW_AT_variable_parameter:
		return "DW_AT_variable_parameter"
	case DW_AT_virtuality:
		return "DW_AT_virtuality"
	case DW_AT_vtable_elem_location:
		return "DW_AT_vtable_elem_location"
	case DW_AT_allocated:
		return "DW_AT_allocated"
	case DW_AT_associated:
		return "DW_AT_associated"
	case DW_AT_data_location:
		return "DW_AT_data_location"
	case DW_AT_byte_stride:
		return "DW_AT_byte_stride"
	case DW_AT_entry_pc:
		return "DW_AT_entry_pc"
	case DW_AT_use_UTF8:
		return "DW_AT_use_UTF8"
	case DW_AT_extension:
		return "DW_AT_extension"
	case DW_AT_ranges:
		return "DW_AT_ranges"
	case DW_AT_trampoline:
		return "DW_AT_trampoline"
	case DW_AT_call_column:
		return "DW_AT_call_column"
	case DW_AT_call_file:
		return "DW_AT_call_file"
	case DW_AT_call_line:
		return "DW_AT_call_line"
	case DW_AT_description:
		return "DW_AT_description"
	case DW_AT_binary_scale:
		return "DW_AT_binary_scale"
	case DW_AT_decimal_scale:
		return "DW_AT_decimal_scale"
	case DW_AT_small:
		return "DW_AT_small"
	case DW_AT_decimal_sign:
		return "DW_AT_decimal_sign"
	case DW_AT_digit_count:
		return "DW_AT_digit_count"
	case DW_AT_picture_string:
		return "DW_AT_picture_string"
	case DW_AT_mutable:
		return "DW_AT_mutable"
	case DW_AT_threads_scaled:
		return "DW_AT_threads_scaled"
	case DW_AT_explicit:
		return "DW_AT_explicit"
	case DW_AT_object_pointer:
		return "DW_AT_object_pointer"
	case DW_AT_endianity:
		return "DW_AT_endianity"
	case DW_AT_elemental:
		return "DW_AT_elemental"
	case DW_AT_pure:
		return "DW_AT_pure"
	case DW_AT_recursive:
		return "DW_AT_recursive"
	case DW_AT_signature:
		return "DW_AT_signature"
	case DW_AT_main_subprogram:
		return "DW_AT_main_subprogram"
	case DW_AT_data_bit_offset:
		return "DW_AT_data_bit_offset"
	case DW_AT_const_expr:
		return "DW_AT_const_expr"
	case DW_AT_enum_class:
		return "DW_AT_enum_class"
	case DW_AT_linkage_name:
		return "DW_AT_linkage_name"
	case DW_AT_defaulted:
		return "DW_AT_defaulted"
	case DW_AT_lo_user:
		return "DW_AT_lo_user"
	case DW_AT_hi_user:
		return "DW_AT_hi_user"
	default:
		return fmt.Sprintf("DW_AT_unknown_%d", attribute)
	}
}
