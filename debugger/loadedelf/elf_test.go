package loadedelf

import (
	"os"
	"testing"

	"github.com/pattyshack/gt/testing/expect"
	"github.com/pattyshack/gt/testing/suite"

	"github.com/pattyshack/bad/elf"
)

// NOTE: the test is in the bad package instead of elf package since the
// test binary is not portable.
type ElfSuite struct{}

func TestElf(t *testing.T) {
	suite.RunTests(t, &ElfSuite{})
}

func (ElfSuite) TestStringTable(t *testing.T) {
	table := elf.NewStringTableSection(
		nil,
		elf.SectionHeaderEntry{},
		[]byte("\x00Milkshake\x00shake\x00no\x00"))

	expect.Equal(t, "Milkshake", table.Get(1))
	expect.Equal(t, "shake", table.Get(5))
	expect.Equal(t, "", table.Get(10))
	expect.Equal(t, "shake", table.Get(11))
	expect.Equal(t, "no", table.Get(17))
	expect.Equal(t, "o", table.Get(18))
	expect.Equal(t, "", table.Get(19))
	expect.Equal(t, "", table.Get(20))
}

func (ElfSuite) TestParse(t *testing.T) {
	content, err := os.ReadFile("../test_targets/hello_world")
	expect.Nil(t, err)

	file, err := elf.ParseBytes(content)
	expect.Nil(t, err)

	section := file.GetSection(".symtab")
	expect.NotNil(t, section)

	table, ok := section.(*elf.SymbolTableSection)
	expect.True(t, ok)

	symbol := table.SymbolAt(elf.FileAddress(file.EntryPointAddress))
	expect.NotNil(t, symbol)
	expect.Equal(t, "_start", symbol.Name)

	symbols := table.SymbolsByName("_start")
	expect.Equal(t, 1, len(symbols))
	expect.Equal(t, "_start", symbols[0].Name)
}
