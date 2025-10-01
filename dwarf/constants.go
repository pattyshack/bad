// NOTE: This is based on based on dwarf.h from github.com/TartanLlama/sdb

package dwarf

const (
	DW_CHILDREN_no  = 0x00
	DW_CHILDREN_yes = 0x01

	DW_DEFAULTED_no           = 0x00
	DW_DEFAULTED_in_class     = 0x01
	DW_DEFAULTED_out_of_class = 0x02

	DW_ATE_address         = 0x01
	DW_ATE_boolean         = 0x02
	DW_ATE_complex_float   = 0x03
	DW_ATE_float           = 0x04
	DW_ATE_signed          = 0x05
	DW_ATE_signed_char     = 0x06
	DW_ATE_unsigned        = 0x07
	DW_ATE_unsigned_char   = 0x08
	DW_ATE_imaginary_float = 0x09
	DW_ATE_packed_decimal  = 0x0a
	DW_ATE_numeric_string  = 0x0b
	DW_ATE_edited          = 0x0c
	DW_ATE_signed_fixed    = 0x0d
	DW_ATE_unsigned_fixed  = 0x0e
	DW_ATE_decimal_float   = 0x0f
	DW_ATE_UTF             = 0x10
	DW_ATE_lo_user         = 0x80
	DW_ATE_hi_user         = 0xff

	DW_DS_unsigned           = 0x01
	DW_DS_leading_overpunch  = 0x02
	DW_DS_trailing_overpunch = 0x03
	DW_DS_leading_separate   = 0x04
	DW_DS_trailing_separate  = 0x05

	DW_END_default = 0x00
	DW_END_big     = 0x01
	DW_END_little  = 0x02
	DW_END_lo_user = 0x40
	DW_END_hi_user = 0xff

	DW_ACCESS_public    = 0x01
	DW_ACCESS_protected = 0x02
	DW_ACCESS_private   = 0x03

	DW_VIS_local     = 0x01
	DW_VIS_exported  = 0x02
	DW_VIS_qualified = 0x03

	DW_VIRTUALITY_none         = 0x00
	DW_VIRTUALITY_virtual      = 0x01
	DW_VIRTUALITY_pure_virtual = 0x02

	DW_LANG_C89            = 0x0001
	DW_LANG_C              = 0x0002
	DW_LANG_Ada83          = 0x0003
	DW_LANG_C_plus_plus    = 0x0004
	DW_LANG_Cobol74        = 0x0005
	DW_LANG_Cobol85        = 0x0006
	DW_LANG_Fortran77      = 0x0007
	DW_LANG_Fortran90      = 0x0008
	DW_LANG_Pascal83       = 0x0009
	DW_LANG_Modula2        = 0x000a
	DW_LANG_Java           = 0x000b
	DW_LANG_C99            = 0x000c
	DW_LANG_Ada95          = 0x000d
	DW_LANG_Fortran95      = 0x000e
	DW_LANG_PLI            = 0x000f
	DW_LANG_ObjC           = 0x0010
	DW_LANG_ObjC_plus_plus = 0x0011
	DW_LANG_UPC            = 0x0012
	DW_LANG_D              = 0x0013
	DW_LANG_Python         = 0x0014
	DW_LANG_lo_user        = 0x8000
	DW_LANG_hi_user        = 0xffff

	DW_ADDR_none = 0x00

	DW_ID_case_sensitive   = 0x00
	DW_ID_up_case          = 0x01
	DW_ID_down_case        = 0x02
	DW_ID_case_insensitive = 0x03

	DW_CC_normal  = 0x01
	DW_CC_program = 0x02
	DW_CC_nocall  = 0x03
	DW_CC_lo_user = 0x40
	DW_CC_hi_user = 0xff

	DW_INL_not_inlined          = 0x00
	DW_INL_inlined              = 0x01
	DW_INL_declared_not_inlined = 0x02
	DW_INL_declared_inlined     = 0x03

	DW_ORD_row_major = 0x00
	DW_ORD_col_major = 0x01

	DW_DSC_label = 0x00
	DW_DSC_range = 0x01

	DW_MACINFO_define     = 0x01
	DW_MACINFO_undef      = 0x02
	DW_MACINFO_start_file = 0x03
	DW_MACINFO_end_file   = 0x04
	DW_MACINFO_vendor_ext = 0xff
)
