package memory

import (
	"fmt"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/ptrace"
)

type VirtualMemory struct {
	processTracer *ptrace.Tracer
}

func New(processTracer *ptrace.Tracer) *VirtualMemory {
	return &VirtualMemory{
		processTracer: processTracer,
	}
}

func (vm *VirtualMemory) Read(addr VirtualAddress, out []byte) (int, error) {
	count, err := vm.processTracer.ReadFromVirtualMemory(uintptr(addr), out)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to read from virtual memory at %s (%d) for process %d: %w",
			addr,
			len(out),
			vm.processTracer.Pid,
			err)
	}

	return count, nil
}

func (vm *VirtualMemory) Write(addr VirtualAddress, data []byte) (int, error) {
	count, err := vm.processTracer.PokeData(uintptr(addr), data)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to write to virtual memory at %s (%d) for process %d: %w",
			addr,
			len(data),
			vm.processTracer.Pid,
			err)
	}

	return count, nil
}
