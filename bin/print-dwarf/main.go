package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/pattyshack/bad/dwarf"
	"github.com/pattyshack/bad/elf"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("USAGE: print-dwarf <file")
		os.Exit(1)
	}

	content, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	elfFile, err := elf.ParseBytes(content)
	if err != nil {
		panic(err)
	}

	file, err := dwarf.NewFile(elfFile)
	if err != nil {
		panic(err)
	}

	entries, err := file.StringSection.StringEntries()
	if err != nil {
		panic(err)
	}

	fmt.Println(".debug_str:")
	for idx, value := range entries {
		fmt.Printf("  %d: %s\n", idx, value)
	}

	fmt.Println(".debug_abbrev:")
	for offset, table := range file.AbbreviationTables {
		fmt.Printf("  table (%d):\n", offset)

		sorted := []*dwarf.Abbreviation{}
		for _, abbrev := range table {
			sorted = append(sorted, abbrev)
		}
		sort.Slice(
			sorted,
			func(i int, j int) bool { return sorted[i].Code < sorted[j].Code })

		for _, abbrev := range sorted {
			fmt.Printf(
				"    Code: %d\tHasChildren: %v\tTag: %s\n",
				abbrev.Code,
				abbrev.HasChildren,
				abbrev.Tag)
			for _, spec := range abbrev.AttributeSpecs {
				fmt.Printf(
					"      Attribute: %s\tFormat: %s\n",
					spec.Attribute,
					spec.Format)
			}
		}
	}

	fmt.Println(".debug_info:")
	for _, unit := range file.CompileUnits {
		entries, err := unit.DebugInfoEntries()
		if err != nil {
			panic(err)
		}

		fmt.Printf(
			"  CompileUnit: Start = %d NumEntries = %d\n",
			unit.Start,
			len(entries))

		root, err := unit.Root()
		if err != nil {
			panic(err)
		}

		printDebugInfoEntry(root, 0)
	}
}

func printDebugInfoEntry(entry *dwarf.DebugInfoEntry, level int) {
	indent := ""
	for i := 0; i < level; i++ {
		indent += "| "
	}

	name, found, err := entry.Name()
	if err != nil {
		panic(err)
	}

	if found {
		name = " (" + name + ")"
	}

	fmt.Printf("    %s%08x: %s%s\n", indent, entry.SectionOffset, entry.Tag, name)
	for idx, spec := range entry.AttributeSpecs {
		fmt.Printf(
			"    %s    %s (%s):\t%v\n",
			indent,
			spec.Attribute,
			spec.Format,
			entry.Values[idx])
	}

	for _, child := range entry.Children {
		printDebugInfoEntry(child, level+1)
	}
}
