package debugger

import (
	"fmt"
	"os"
	"path"
	"strings"

	. "github.com/pattyshack/bad/debugger/common"
)

type Snippet struct {
	Start int // 1-based
	Focus int // 1-based
	End   int // exclusive
	Lines []string
}

func (snippet Snippet) String() string {
	template := fmt.Sprintf("%%s %%%dd %%s", len(fmt.Sprintf("%d", snippet.End)))

	content := []string{}
	for idx, line := range snippet.Lines {
		current := snippet.Start + idx
		prefix := " "
		if current == snippet.Focus {
			prefix = ">"
		}

		content = append(content, fmt.Sprintf(template, prefix, current, line))
	}

	return strings.Join(content, "\n")
}

type SourceFiles struct {
	Files map[string][]string // XXX: LRU?
}

func NewSourceFiles() *SourceFiles {
	return &SourceFiles{
		Files: map[string][]string{},
	}
}

func (files *SourceFiles) GetSnippet(
	pathName string,
	focus int,
	delta int,
) (
	Snippet,
	error,
) {
	pathName = path.Clean(pathName)
	lines, ok := files.Files[pathName]
	if !ok {
		content, err := os.ReadFile(pathName)
		if err != nil {
			return Snippet{}, fmt.Errorf("failed to read %s: %w", pathName, err)
		}

		lines = strings.Split(string(content), "\n")
		files.Files[pathName] = lines
	}

	if focus <= 0 || focus > len(lines) {
		return Snippet{}, fmt.Errorf(
			"%w. out of bound focus line",
			ErrInvalidInput)
	}

	if delta < 0 {
		return Snippet{}, fmt.Errorf("%w. negative line delta", ErrInvalidInput)
	}

	startLine := focus - delta
	startIdx := startLine - 1
	if startIdx < 0 {
		startLine = 1
		startIdx = 0
	}

	endLine := focus + delta + 1
	endIdx := endLine - 1
	if endIdx >= len(lines) {
		endLine = len(lines) + 1
		endIdx = len(lines)
	}

	return Snippet{
		Start: startLine,
		Focus: focus,
		End:   endLine,
		Lines: lines[startIdx:endIdx],
	}, nil
}
