package procfs

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type ProcessState string

const (
	Running        = ProcessState("running")
	Sleeping       = ProcessState("sleeping")
	WaitingForDisk = ProcessState("waiting for disk")
	Zombie         = ProcessState("zombie")
	TracingStop    = ProcessState("tracing stop")
	Dead           = ProcessState("dead")
	Idle           = ProcessState("idle")
)

type ProcessStatus struct {
	Pid   int
	Comm  string
	State ProcessState
	Ppid  int
	Pgrp  int

	// NOTE: See man page for the full list of (52) fields.
}

func GetProcessStatus(pid int) (ProcessStatus, error) {
	contentBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return ProcessStatus{}, fmt.Errorf(
			"failed to read process %d status: %w",
			pid,
			err)
	}

	content := string(contentBytes)

	commStart := strings.Index(content, "(")
	commEnd := strings.LastIndex(content, ")")

	chunks := strings.Split(content[commEnd+2:], " ")

	pid, err = strconv.Atoi(strings.TrimSpace(content[:commStart]))
	if err != nil {
		panic("should never happen: " + err.Error())
	}

	var state ProcessState
	switch chunks[0] {
	case "R":
		state = Running
	case "S":
		state = Sleeping
	case "D":
		state = WaitingForDisk
	case "Z":
		state = Zombie
	case "t":
		state = TracingStop
	case "X":
		state = Dead
	case "I":
		state = Idle
	}

	ppid, err := strconv.Atoi(chunks[1])
	if err != nil {
		panic("should never happen: " + err.Error())
	}

	pgrp, err := strconv.Atoi(chunks[2])
	if err != nil {
		panic("should never happen: " + err.Error())
	}

	return ProcessStatus{
		Pid:   pid,
		Comm:  content[commStart+1 : commEnd],
		State: state,
		Ppid:  ppid,
		Pgrp:  pgrp,
	}, nil
}

// See elf.h for the full list of auxiliary vector entry types, system v abi
// amd64 supplement section 3.4.3 for description.
type AuxiliaryVectorEntryType uint64

const (
	// AT_NULL. last entry of the vector
	AT_EndOfVector = AuxiliaryVectorEntryType(0)

	// AT_IGNORE. entry with no meaning
	AT_Ignore = AuxiliaryVectorEntryType(1)

	// NOTE: The system only sets one of ExecFd or ProgramHeader, but not both.
	AT_ExecFd        = AuxiliaryVectorEntryType(2) // AT_EXECFD
	AT_ProgramHeader = AuxiliaryVectorEntryType(3) // AT_PHDR

	// AT_PHENT. size of one program header entry in bytes
	AT_ProgramHeaderEntrySize = AuxiliaryVectorEntryType(4)

	// AT_PHNUM
	AT_NumProgramHeaderEntries = AuxiliaryVectorEntryType(5)

	// AT_PAGESZ. system page size in bytes
	AT_PageSize = AuxiliaryVectorEntryType(6)

	// AT_BASE. base address at which the interpreter program was loaded into
	// memory.
	AT_BaseAddress = AuxiliaryVectorEntryType(7)

	// AT_FLAGS
	AT_Flags = AuxiliaryVectorEntryType(8)

	// AT_ENTRY. entry point of the application program
	AT_Entry = AuxiliaryVectorEntryType(9)

	// AT_NOTELF. non-zero if the program is not in elf format
	AT_NotElf = AuxiliaryVectorEntryType(10)

	// AT_UID. process' real user id
	AT_UID = AuxiliaryVectorEntryType(11)

	// AT_EUID. process' effective user id
	AT_EUID = AuxiliaryVectorEntryType(12)

	// AT_GID. process' real group id
	AT_GID = AuxiliaryVectorEntryType(13)

	// AT_GID. process' effective group id
	AT_EGID = AuxiliaryVectorEntryType(14)
)

// NOTE: access to this is governed by ptrace
func GetAuxiliaryVector(pid int) (map[AuxiliaryVectorEntryType]uint64, error) {
	content, err := os.ReadFile(fmt.Sprintf("/proc/%d/auxv", pid))
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read process %d's auxiliary vector: %w",
			pid,
			err)
	}

	result := map[AuxiliaryVectorEntryType]uint64{}
	for {
		var avet AuxiliaryVectorEntryType
		n, err := binary.Decode(content, binary.LittleEndian, &avet)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to decode process %d's auxiliary vector: %w",
				pid,
				err)
		}
		if n != 8 {
			panic("should never happen")
		}
		content = content[8:]

		if avet == AT_EndOfVector {
			break
		}

		var value uint64
		n, err = binary.Decode(content, binary.LittleEndian, &value)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to decode process %d's auxiliary vector: %w",
				pid,
				err)
		}
		if n != 8 {
			panic("should never happen")
		}
		content = content[8:]

		if avet == AT_Ignore {
			continue
		}

		result[avet] = value
	}

	return result, nil
}

type MappedMemoryRegion struct {
	LowAddress  uint64
	HighAddress uint64

	Read    bool
	Write   bool
	Execute bool
	Private bool // (copy on write)

	Offset uint64

	DeviceMajor uint
	DeviceMinor uint
	Inode       uint

	Pathname string
}

func GetMappedMemoryRegions(pid int) ([]MappedMemoryRegion, error) {
	path := fmt.Sprintf("/proc/%d/maps", pid)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	result := []MappedMemoryRegion{}
	for _, line := range strings.Split(string(content), "\n") {
		if line == "" {
			break
		}

		entry := MappedMemoryRegion{}
		chunks := strings.SplitN(line, " ", 6)

		addresses := strings.SplitN(chunks[0], "-", 2)

		lowAddr, err := strconv.ParseUint(addresses[0], 16, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse low address: %w", err)
		}
		entry.LowAddress = lowAddr

		highAddr, err := strconv.ParseUint(addresses[1], 16, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse high address: %w", err)
		}
		entry.HighAddress = highAddr

		for idx, b := range []byte(chunks[1]) {
			switch idx {
			case 0:
				entry.Read = b == 'r'
			case 1:
				entry.Write = b == 'w'
			case 2:
				entry.Execute = b == 'x'
			case 3:
				entry.Private = b == 'p'
			}
		}

		offset, err := strconv.ParseUint(chunks[2], 16, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse offset: %w", err)
		}
		entry.Offset = offset

		device := strings.SplitN(chunks[3], ":", 2)

		major, err := strconv.ParseUint(device[0], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse device major: %w", err)
		}
		entry.DeviceMajor = uint(major)

		minor, err := strconv.ParseUint(device[1], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse device minor: %w", err)
		}
		entry.DeviceMinor = uint(minor)

		inode, err := strconv.ParseUint(chunks[4], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse inode: %w", err)
		}
		entry.Inode = uint(inode)

		if len(chunks) == 6 {
			entry.Pathname = strings.TrimSpace(chunks[5])
		}

		result = append(result, entry)
	}

	return result, nil
}

func GetExecutableSymlinkPath(pid int) string {
	return fmt.Sprintf("/proc/%d/exe", pid)
}
