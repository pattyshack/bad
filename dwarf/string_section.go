package dwarf

import (
	"bytes"
	"fmt"

	"github.com/pattyshack/bad/elf"
)

type StringSection struct {
	found   bool
	content []byte
}

func NewStringSection(file *elf.File) (*StringSection, error) {
	section := file.GetSection(ElfDebugStringSection)

	var content []byte
	if section != nil {
		var err error
		content, err = section.RawContent()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to read %s section from elf: %w",
				ElfDebugStringSection,
				err)
		}
	}

	return &StringSection{
		found:   section != nil,
		content: content,
	}, nil
}

func (table *StringSection) StringAt(offset SectionOffset) (string, error) {
	value, _, err := table.getStringAt(int(offset))
	return value, err
}

func (table *StringSection) getStringAt(offset int) (string, int, error) {
	if !table.found {
		return "", 0, fmt.Errorf("elf .debug_str section not found")
	}

	if offset < 0 || len(table.content) <= offset {
		return "", 0, fmt.Errorf("out of bound string reference (%d)", offset)
	}

	content := table.content[offset:]
	end := bytes.IndexByte(content, 0)
	if end == -1 {
		return "", 0, fmt.Errorf("string reference not terminated")
	}

	return string(content[:end]), offset + end + 1, nil
}

func (table *StringSection) StringEntries() ([]string, error) {
	result := []string{}
	offset := 0
	for len(table.content) > offset {
		value, next, err := table.getStringAt(offset)
		if err != nil {
			return nil, err
		}

		result = append(result, value)
		offset = next
	}

	return result, nil
}
