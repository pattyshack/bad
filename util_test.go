package bad

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/pattyshack/bad/elf"
)

type Elf64FileAddress int64

func getSectionLoadBias(path string, fileAddr Elf64FileAddress) (int64, error) {
	content, err := exec.Command("readelf", "-WS", path).Output()
	if err != nil {
		return 0, fmt.Errorf("failed to readelf: %w", err)
	}

	re := regexp.MustCompile(`PROGBITS\s+(\w+)\s+(\w+)\s+(\w+)`)
	for _, line := range strings.Split(string(content), "\n") {
		match := re.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		lowAddr, err := strconv.ParseInt(match[0], 16, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse address: %w", err)
		}

		offset, err := strconv.ParseInt(match[2], 16, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse offset: %w", err)
		}

		size, err := strconv.ParseInt(match[3], 16, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse size: %w", err)
		}

		if lowAddr <= int64(fileAddr) && int64(fileAddr) < (lowAddr+size) {
			return lowAddr - offset, nil
		}
	}

	panic("should never happen")
}

func getEntryPointOffset(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer file.Close()

	headerBytes := make([]byte, elf.Elf64HeaderSize)
	n, err := io.ReadFull(file, headerBytes)
	if err != nil {
		return 0, fmt.Errorf("failed to read elf header: %w", err)
	}
	if n != len(headerBytes) {
		return 0, fmt.Errorf(
			"failed to read elf header for %s. wrong size (%d != %d)",
			path,
			n,
			len(headerBytes))
	}

	hdr := elf.ElfHeader{}
	n, err = binary.Decode(headerBytes, binary.NativeEndian, &hdr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse elf header: %w", err)
	}
	if n != len(headerBytes) {
		panic("should never happen")
	}

	return int64(hdr.EntryPointAddress), nil
}

func getLoadAddress(pid int, offset int64) (VirtualAddress, error) {
	path := fmt.Sprintf("/proc/%d/maps", pid)
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s: %w", path, err)
	}

	re := regexp.MustCompile(`(\w+)-\w+ ..(.). (\w+)`)
	for _, line := range strings.Split(string(content), "\n") {
		match := re.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		if match[2] != "x" { // virtual memory section is not executable code
			continue
		}

		lowAddr, err := strconv.ParseInt(match[1], 16, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse address: %w", err)
		}

		fileOffset, err := strconv.ParseInt(match[3], 16, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse file offset: %w", err)
		}

		return VirtualAddress(offset - fileOffset + lowAddr), nil
	}

	panic("should never happen")
}
