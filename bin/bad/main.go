package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/chzyer/readline"

	"github.com/pattyshack/bad"
)

type command struct {
	name string
	run  func([]string) error
}

var (
	commands = []command{
		{
			name: "continue",
			run:  bad.Resume,
		},
	}
)

func main() {
	rl, err := readline.New("bad > ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	lastLine := ""
	for {
		line, err := rl.Readline()
		if err != nil {
			if err == io.EOF {
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
				err := cmd.run(args[1:])
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
