package main

import (
	"fmt"
	"os"

	"github.com/pattyshack/bad/elf"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("USAGE: print-elf <file")
		os.Exit(1)
	}

	content, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	file, err := elf.ParseBytes(content)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Header: %v\n", file.ElfHeader)

	fmt.Println("Sections:", len(file.Sections))
	for sectionIdx, section := range file.Sections {
		fmt.Printf("  [%d] %s: %v\n", sectionIdx, section.Name(), section.Header())

		switch s := section.(type) {
		case *elf.StringTableSection:
			fmt.Printf("    Number of string entries: %d\n", s.NumEntries())
		case *elf.SymbolTableSection:
			for symbolIdx, entry := range s.Symbols {
				fmt.Printf(
					"    %d: %x %d %s %s %s %d %s\n",
					symbolIdx,
					entry.Value,
					entry.Size,
					entry.Type(),
					entry.Binding(),
					entry.SymbolVisibility,
					entry.SectionIndex,
					entry.PrettyName())
			}
		case *elf.NoteSection:
			for noteIdx, entry := range s.Entries {
				fmt.Printf(
					"    %d: Name = %s Type = %d Description length = %d\n",
					noteIdx,
					entry.Name,
					entry.Type,
					len(entry.Description))
			}
		}
	}

	fmt.Println("Program headers:", len(file.ProgramHeaders))
	for headerIdx, header := range file.ProgramHeaders {
		fmt.Printf("  [%d] %v\n", headerIdx, header)
	}
}
