package main

import (
	"fmt"
	"strconv"

	"github.com/pattyshack/bad"
)

func readMemory(db *bad.Debugger, args []string) error {
	if len(args) == 0 {
		fmt.Println("failed to read from memory. address not specified")
		return nil
	}

	addr, err := strconv.ParseUint(args[0], 0, 64)
	if err != nil {
		fmt.Println("failed to parse memory address:", err)
		return nil
	}

	size := 32
	if len(args) > 1 {
		val, err := strconv.ParseInt(args[1], 0, 32)
		if err != nil {
			fmt.Println("failed to parse output size:", err)
			return nil
		}
		size = int(val)

		if size < 1 {
			fmt.Println("invalid output size:", size)
			return nil
		}
	}

	out := make([]byte, size)
	numRead, err := db.ReadFromVirtualMemory(bad.VirtualAddress(addr), out)
	if err != nil {
		fmt.Println("failed to read from memory:", err)
		return nil
	}

	if numRead < size {
		fmt.Printf(
			"WARNING: requested %d bytes but only read %d bytes.\n",
			size,
			numRead)
	}

	for len(out) > 0 {
		line := fmt.Sprintf("0x%016x:", addr)

		size = 16
		if len(out) < size {
			size = len(out)
		}

		for _, b := range out[:size] {
			line += fmt.Sprintf(" %02x", b)
		}
		fmt.Println(line)

		out = out[size:]
		addr += uint64(size)
	}

	return nil
}

func writeMemory(db *bad.Debugger, args []string) error {
	if len(args) == 0 {
		fmt.Println("failed to write to memory. address not specified.")
		return nil
	}

	addr, err := strconv.ParseUint(args[0], 0, 64)
	if err != nil {
		fmt.Println("failed to parse memory address:", err)
		return nil
	}

	data := []byte{}
	for idx, arg := range args[1:] {
		val, err := strconv.ParseUint(arg, 0, 8)
		if err != nil {
			fmt.Printf(
				"failed to parse byte at argument %d: %s\n",
				idx+1,
				err)
			return nil
		}

		data = append(data, byte(val))
	}

	if len(data) == 0 {
		fmt.Println("failed to write to memory. no bytes specified.")
		return nil
	}

	numWritten, err := db.WriteToVirtualMemory(bad.VirtualAddress(addr), data)
	if err != nil {
		fmt.Println("failed to write to memory:", err)
		return nil
	}

	if numWritten < len(data) {
		fmt.Printf(
			"WARNING: provided %d bytes but only written %d bytes.\n",
			len(data),
			numWritten)
	}

	return nil
}
