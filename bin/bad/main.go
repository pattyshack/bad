package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/chzyer/readline"

	"github.com/pattyshack/bad"
)

type command struct {
	name string
	run  func(*bad.Debugger, []string) error
}

var (
	commands = []command{
		{
			name: "continue",
			run:  bad.Continue,
		},
	}
)

func main() {
	pid := 0
	flag.IntVar(&pid, "p", 0, "attach to existing process pid")

	flag.Parse()
	args := flag.Args()

	var db *bad.Debugger
	var err error
	if pid != 0 {
		if len(args) != 0 {
			panic("unexpected arguments")
		}

		db, err = bad.AttachToProcess(pid)
	} else if len(args) == 0 {
		panic("no arguments given")
	} else {
		db, err = bad.StartAndAttachToProcess(args[0], args[1:]...)
	}

	if err != nil {
		panic(err)
	}

	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	fmt.Println("attached to process", db.Pid)

	rl, err := readline.New("bad > ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	lastLine := ""
	for {
		line, err := rl.Readline()
		if err != nil {
			if err == io.EOF || err == readline.ErrInterrupt {
				break
			}
			panic(err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			line = lastLine
		}
		lastLine = line

		if line == "" {
			continue
		}

		args := strings.Split(line, " ")
		if args[0] == "" {
			fmt.Println("invalid command: (empty string)")
		}

		found := false
		for _, cmd := range commands {
			if strings.HasPrefix(cmd.name, args[0]) {
				found = true
				err := cmd.run(db, args[1:])
				if err != nil {
					panic(err)
				}
			}
		}

		if !found {
			fmt.Println("invalid command:", args[0])
		}
	}
}
