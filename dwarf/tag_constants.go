// NOTE: This is based on based on dwarf.h from github.com/TartanLlama/sdb

package dwarf

import (
	"fmt"
)

// See dwarf 5 table 7.3 for full list
type Tag uint64

const (
	DW_TAG_array_type               = Tag(0x01)
	DW_TAG_class_type               = Tag(0x02)
	DW_TAG_entry_point              = Tag(0x03)
	DW_TAG_enumeration_type         = Tag(0x04)
	DW_TAG_formal_parameter         = Tag(0x05)
	DW_TAG_imported_declaration     = Tag(0x08)
	DW_TAG_label                    = Tag(0x0a)
	DW_TAG_lexical_block            = Tag(0x0b)
	DW_TAG_member                   = Tag(0x0d)
	DW_TAG_pointer_type             = Tag(0x0f)
	DW_TAG_reference_type           = Tag(0x10)
	DW_TAG_compile_unit             = Tag(0x11)
	DW_TAG_string_type              = Tag(0x12)
	DW_TAG_structure_type           = Tag(0x13)
	DW_TAG_subroutine_type          = Tag(0x15)
	DW_TAG_typedef                  = Tag(0x16)
	DW_TAG_union_type               = Tag(0x17)
	DW_TAG_unspecified_parameters   = Tag(0x18)
	DW_TAG_variant                  = Tag(0x19)
	DW_TAG_common_block             = Tag(0x1a)
	DW_TAG_common_inclusion         = Tag(0x1b)
	DW_TAG_inheritance              = Tag(0x1c)
	DW_TAG_inlined_subroutine       = Tag(0x1d)
	DW_TAG_module                   = Tag(0x1e)
	DW_TAG_ptr_to_member_type       = Tag(0x1f)
	DW_TAG_set_type                 = Tag(0x20)
	DW_TAG_subrange_type            = Tag(0x21)
	DW_TAG_with_stmt                = Tag(0x22)
	DW_TAG_access_declaration       = Tag(0x23)
	DW_TAG_base_type                = Tag(0x24)
	DW_TAG_catch_block              = Tag(0x25)
	DW_TAG_const_type               = Tag(0x26)
	DW_TAG_constant                 = Tag(0x27)
	DW_TAG_enumerator               = Tag(0x28)
	DW_TAG_file_type                = Tag(0x29)
	DW_TAG_friend                   = Tag(0x2a)
	DW_TAG_namelist                 = Tag(0x2b)
	DW_TAG_namelist_item            = Tag(0x2c)
	DW_TAG_packed_type              = Tag(0x2d)
	DW_TAG_subprogram               = Tag(0x2e)
	DW_TAG_template_type_parameter  = Tag(0x2f)
	DW_TAG_template_value_parameter = Tag(0x30)
	DW_TAG_thrown_type              = Tag(0x31)
	DW_TAG_try_block                = Tag(0x32)
	DW_TAG_variant_part             = Tag(0x33)
	DW_TAG_variable                 = Tag(0x34)
	DW_TAG_volatile_type            = Tag(0x35)
	DW_TAG_dwarf_procedure          = Tag(0x36)
	DW_TAG_restrict_type            = Tag(0x37)
	DW_TAG_interface_type           = Tag(0x38)
	DW_TAG_namespace                = Tag(0x39)
	DW_TAG_imported_module          = Tag(0x3a)
	DW_TAG_unspecified_type         = Tag(0x3b)
	DW_TAG_partial_unit             = Tag(0x3c)
	DW_TAG_imported_unit            = Tag(0x3d)
	DW_TAG_condition                = Tag(0x3f)
	DW_TAG_shared_type              = Tag(0x40)
	DW_TAG_type_unit                = Tag(0x41)
	DW_TAG_rvalue_reference_type    = Tag(0x42)
	DW_TAG_template_alias           = Tag(0x43)
	DW_TAG_lo_user                  = Tag(0x4080)
	DW_TAG_hi_user                  = Tag(0xffff)
)

func (tag Tag) String() string {
	switch tag {
	case DW_TAG_array_type:
		return "DW_TAG_array_type"
	case DW_TAG_class_type:
		return "DW_TAG_class_type"
	case DW_TAG_entry_point:
		return "DW_TAG_entry_point"
	case DW_TAG_enumeration_type:
		return "DW_TAG_enumeration_type"
	case DW_TAG_formal_parameter:
		return "DW_TAG_formal_parameter"
	case DW_TAG_imported_declaration:
		return "DW_TAG_imported_declaration"
	case DW_TAG_label:
		return "DW_TAG_label"
	case DW_TAG_lexical_block:
		return "DW_TAG_lexical_block"
	case DW_TAG_member:
		return "DW_TAG_member"
	case DW_TAG_pointer_type:
		return "DW_TAG_pointer_type"
	case DW_TAG_reference_type:
		return "DW_TAG_reference_type"
	case DW_TAG_compile_unit:
		return "DW_TAG_compile_unit"
	case DW_TAG_string_type:
		return "DW_TAG_string_type"
	case DW_TAG_structure_type:
		return "DW_TAG_structure_type"
	case DW_TAG_subroutine_type:
		return "DW_TAG_subroutine_type"
	case DW_TAG_typedef:
		return "DW_TAG_typedef"
	case DW_TAG_union_type:
		return "DW_TAG_union_type"
	case DW_TAG_unspecified_parameters:
		return "DW_TAG_unspecified_parameters"
	case DW_TAG_variant:
		return "DW_TAG_variant"
	case DW_TAG_common_block:
		return "DW_TAG_common_block"
	case DW_TAG_common_inclusion:
		return "DW_TAG_common_inclusion"
	case DW_TAG_inheritance:
		return "DW_TAG_inheritance"
	case DW_TAG_inlined_subroutine:
		return "DW_TAG_inlined_subroutine"
	case DW_TAG_module:
		return "DW_TAG_module"
	case DW_TAG_ptr_to_member_type:
		return "DW_TAG_ptr_to_member_type"
	case DW_TAG_set_type:
		return "DW_TAG_set_type"
	case DW_TAG_subrange_type:
		return "DW_TAG_subrange_type"
	case DW_TAG_with_stmt:
		return "DW_TAG_with_stmt"
	case DW_TAG_access_declaration:
		return "DW_TAG_access_declaration"
	case DW_TAG_base_type:
		return "DW_TAG_base_type"
	case DW_TAG_catch_block:
		return "DW_TAG_catch_block"
	case DW_TAG_const_type:
		return "DW_TAG_const_type"
	case DW_TAG_constant:
		return "DW_TAG_constant"
	case DW_TAG_enumerator:
		return "DW_TAG_enumerator"
	case DW_TAG_file_type:
		return "DW_TAG_file_type"
	case DW_TAG_friend:
		return "DW_TAG_friend"
	case DW_TAG_namelist:
		return "DW_TAG_namelist"
	case DW_TAG_namelist_item:
		return "DW_TAG_namelist_item"
	case DW_TAG_packed_type:
		return "DW_TAG_packed_type"
	case DW_TAG_subprogram:
		return "DW_TAG_subprogram"
	case DW_TAG_template_type_parameter:
		return "DW_TAG_template_type_parameter"
	case DW_TAG_template_value_parameter:
		return "DW_TAG_template_value_parameter"
	case DW_TAG_thrown_type:
		return "DW_TAG_thrown_type"
	case DW_TAG_try_block:
		return "DW_TAG_try_block"
	case DW_TAG_variant_part:
		return "DW_TAG_variant_part"
	case DW_TAG_variable:
		return "DW_TAG_variable"
	case DW_TAG_volatile_type:
		return "DW_TAG_volatile_type"
	case DW_TAG_dwarf_procedure:
		return "DW_TAG_dwarf_procedure"
	case DW_TAG_restrict_type:
		return "DW_TAG_restrict_type"
	case DW_TAG_interface_type:
		return "DW_TAG_interface_type"
	case DW_TAG_namespace:
		return "DW_TAG_namespace"
	case DW_TAG_imported_module:
		return "DW_TAG_imported_module"
	case DW_TAG_unspecified_type:
		return "DW_TAG_unspecified_type"
	case DW_TAG_partial_unit:
		return "DW_TAG_partial_unit"
	case DW_TAG_imported_unit:
		return "DW_TAG_imported_unit"
	case DW_TAG_condition:
		return "DW_TAG_condition"
	case DW_TAG_shared_type:
		return "DW_TAG_shared_type"
	case DW_TAG_type_unit:
		return "DW_TAG_type_unit"
	case DW_TAG_rvalue_reference_type:
		return "DW_TAG_rvalue_reference_type"
	case DW_TAG_template_alias:
		return "DW_TAG_template_alias"
	case DW_TAG_lo_user:
		return "DW_TAG_lo_user"
	case DW_TAG_hi_user:
		return "DW_TAG_hi_user"
	default:
		return fmt.Sprintf("DW_TAG_unknown_%d", tag)
	}
}
