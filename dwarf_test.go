package bad

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/pattyshack/gt/testing/expect"
	"github.com/pattyshack/gt/testing/suite"

	"github.com/pattyshack/bad/dwarf"
	"github.com/pattyshack/bad/elf"
)

// NOTE: the test is in the bad package instead of dwarf package since the
// test binary is not portable.
type DwarfSuite struct{}

func TestDwarf(t *testing.T) {
	suite.RunTests(t, &DwarfSuite{})
}

func (DwarfSuite) newFile(t *testing.T, path string) *dwarf.File {
	content, err := os.ReadFile("test_targets/hello_world")
	expect.Nil(t, err)

	elfFile, err := elf.ParseBytes(content)
	expect.Nil(t, err)

	file, err := dwarf.NewFile(elfFile)
	expect.Nil(t, err)

	return file
}

func (s DwarfSuite) TestParseBasic(t *testing.T) {
	file := s.newFile(t, "test_targets/hello_world")

	expect.Equal(t, 1, len(file.CompileUnits))
	unit := file.CompileUnits[0]

	root, err := unit.Root()
	expect.Nil(t, err)

	lang, ok := root.Uint(dwarf.DW_AT_language)
	expect.True(t, ok)
	expect.Equal(t, dwarf.DW_LANG_C_plus_plus, lang)

	expect.True(t, len(root.Children) > 0)
}

func (s DwarfSuite) TestFindFunction(t *testing.T) {
	file := s.newFile(t, "test_targets/multi_cu")

	// XXX: gcc compiled multi_cu into a single compile unit ....
	// expect.True(t, len(file.CompileUnits) > 1)

	entries, err := file.FunctionEntriesWithName("main")
	expect.Nil(t, err)
	expect.True(t, len(entries) > 0)

	expect.Equal(t, dwarf.DW_TAG_subprogram, entries[0].Tag)

	name, ok := entries[0].String(dwarf.DW_AT_name)
	expect.True(t, ok)
	expect.Equal(t, "main", name)
}

func (DwarfSuite) TestAddressRanges(t *testing.T) {
	// NOTE: the data is big endian encoded
	content := []byte{
		// junk
		0x0b, 0x0a, 0x0d, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x0b, 0x0a, 0x0d, 0x00, 0x00, 0x00, 0x00, 0x00,

		// range entries
		0x01, 0x02, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x02, 0x03, 0x14, 0x00, 0x00, 0x00, 0x00,

		0x02, 0x03, 0x04, 0x05, 0x00, 0x00, 0x00, 0x00,
		0x02, 0x03, 0x04, 0x25, 0x00, 0x00, 0x00, 0x00,

		// new base address
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x00, 0x00, 0x00, 0x08, 0x07, 0x06, 0x05,

		// range entries
		0x01, 0x02, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x02, 0x03, 0x14, 0x00, 0x00, 0x00, 0x00,

		0x02, 0x03, 0x04, 0x05, 0x00, 0x00, 0x00, 0x00,
		0x02, 0x03, 0x04, 0x25, 0x00, 0x00, 0x00, 0x00,

		// end of list
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

		// junk
		0x0b, 0x0a, 0x0d, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x0b, 0x0a, 0x0d, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	ar := dwarf.NewAddressRangesSectionFromBytes(binary.BigEndian, content)

	addressRanges, err := ar.AddressRangesAt(16, 0x05060708)
	expect.Nil(t, err)
	expect.Equal(t, 4, len(addressRanges))

	expect.Equal(
		t,
		dwarf.AddressRange{
			Low:  0x0102030405060708,
			High: 0x0102031405060708,
		},
		addressRanges[0])
	expect.Equal(
		t,
		dwarf.AddressRange{
			Low:  0x0203040505060708,
			High: 0x0203042505060708,
		},
		addressRanges[1])
	expect.Equal(
		t,
		dwarf.AddressRange{
			Low:  0x0102030408070605,
			High: 0x0102031408070605,
		},
		addressRanges[2])
	expect.Equal(
		t,
		dwarf.AddressRange{
			Low:  0x0203040508070605,
			High: 0x0203042508070605,
		},
		addressRanges[3])
}

func (s DwarfSuite) TestLineTable(t *testing.T) {
	file := s.newFile(t, "test_targets/hello_world")

	expect.Equal(t, 1, len(file.CompileUnits))

	iter, err := file.CompileUnits[0].LineIterator()
	expect.Nil(t, err)
	expect.NotNil(t, iter)

	entry1, err := iter.Next()
	expect.Nil(t, err)
	expect.NotNil(t, entry1)
	expect.Equal(t, "hello_world.cpp", entry1.Name)
	expect.Equal(t, 3, entry1.Line)
	expect.False(t, entry1.EndSequence)

	entry2, err := iter.Next()
	expect.Nil(t, err)
	expect.NotNil(t, entry2)
	expect.Equal(t, "hello_world.cpp", entry2.Name)
	expect.Equal(t, 4, entry2.Line)
	expect.False(t, entry2.EndSequence)

	entry3, err := iter.Next()
	expect.Nil(t, err)
	expect.NotNil(t, entry3)
	expect.Equal(t, "hello_world.cpp", entry3.Name)
	expect.Equal(t, 5, entry3.Line)
	expect.False(t, entry3.EndSequence)

	entry4, err := iter.Next()
	expect.Nil(t, err)
	expect.NotNil(t, entry4)
	expect.Equal(t, "hello_world.cpp", entry4.Name)
	expect.Equal(t, 5, entry4.Line)
	expect.True(t, entry4.EndSequence)

	entry5, err := iter.Next()
	expect.Nil(t, err)
	expect.Nil(t, entry5)

	// test resuming iteration from state snapshots

	for i := 0; i < 3; i++ {
		iter = entry2.Resume()

		entry3, err = iter.Next()
		expect.Nil(t, err)
		expect.NotNil(t, entry3)
		expect.Equal(t, "hello_world.cpp", entry3.Name)
		expect.Equal(t, 5, entry3.Line)
		expect.False(t, entry3.EndSequence)

		entry4, err = iter.Next()
		expect.Nil(t, err)
		expect.NotNil(t, entry4)
		expect.Equal(t, "hello_world.cpp", entry4.Name)
		expect.Equal(t, 5, entry4.Line)
		expect.True(t, entry4.EndSequence)

		entry5, err = iter.Next()
		expect.Nil(t, err)
		expect.Nil(t, entry5)
	}

	for i := 0; i < 3; i++ {
		iter = entry1.Resume()

		entry2, err = iter.Next()
		expect.Nil(t, err)
		expect.NotNil(t, entry2)
		expect.Equal(t, "hello_world.cpp", entry2.Name)
		expect.Equal(t, 4, entry2.Line)
		expect.False(t, entry2.EndSequence)

		entry3, err = iter.Next()
		expect.Nil(t, err)
		expect.NotNil(t, entry3)
		expect.Equal(t, "hello_world.cpp", entry3.Name)
		expect.Equal(t, 5, entry3.Line)
		expect.False(t, entry3.EndSequence)

		entry4, err = iter.Next()
		expect.Nil(t, err)
		expect.NotNil(t, entry4)
		expect.Equal(t, "hello_world.cpp", entry4.Name)
		expect.Equal(t, 5, entry4.Line)
		expect.True(t, entry4.EndSequence)

		entry5, err = iter.Next()
		expect.Nil(t, err)
		expect.Nil(t, entry5)
	}
}
