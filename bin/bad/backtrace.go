package main

import (
	"fmt"
	"path"
	"strings"

	"github.com/pattyshack/bad/debugger"
)

func backtrace(db *debugger.Debugger, args string) error {
	args = strings.TrimSpace(args)
	if strings.HasPrefix("up", args) {
		db.InspectCalleeFrame()
	} else if strings.HasPrefix("down", args) {
		db.InspectCallerFrame()
	}

	inspectFrame, backtraceStack := db.BacktraceStack()

	fmt.Println("Backtrace:")
	for idx, frame := range backtraceStack {
		prefix := "  "
		if inspectFrame == frame {
			prefix = " *"
		}

		inlinedStr := ""
		if frame.IsInlined() {
			inlinedStr = fmt.Sprintf("(inlined in %s) ", frame.BaseFrame.Name)
		}

		libStr := ""
		elfFileName := ""
		if frame.SourceFile != nil {
			elfFileName = frame.SourceFile.CompileUnit.File.FileName
		}
		if elfFileName != "" {
			libStr = fmt.Sprintf(" [%s]", path.Base(elfFileName))
		}

		fmt.Printf(
			"%s%2d. %s %s%s\n",
			prefix,
			idx,
			frame.BacktraceProgramCounter,
			inlinedStr,
			frame.Name)
		fmt.Printf("        %s:%d%s\n", frame.SourceFile, frame.SourceLine, libStr)
	}

	return nil
}
