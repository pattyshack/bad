package bad

import (
	"fmt"
)

const (
	int3Instruction = byte(0xcc)
)

func SoftwareBreakPointSiteOptions() StopPointOptions {
	return StopPointOptions{
		Type: StopPointType{
			Kind:      SoftwareKind,
			Mode:      ExecuteMode,
			WatchSize: 1,
		},
	}
}

type softwareBreakPointSiteAllocator struct {
	debugger *Debugger
}

func (allocator *softwareBreakPointSiteAllocator) SetDebugger(
	debugger *Debugger,
) {
	allocator.debugger = debugger
}

func (allocator *softwareBreakPointSiteAllocator) Allocate(
	address VirtualAddress,
	options StopPointOptions,
) (
	StopPoint,
	error,
) {
	if options.Type.Kind != SoftwareKind {
		return nil, fmt.Errorf(
			"%w. invalid stop point kind (%s)",
			ErrInvalidArgument,
			options.Type.Kind)
	}

	err := options.Type.Validate(address)
	if err != nil {
		return nil, err
	}

	return &SoftwareBreakPointSite{
		debugger:      allocator.debugger,
		StopPointType: options.Type,
		address:       address,
		isEnabled:     false,
		originalData:  0,
	}, nil
}

type SoftwareBreakPointSite struct {
	debugger *Debugger

	StopPointType

	address      VirtualAddress
	isEnabled    bool
	originalData byte
}

func (site *SoftwareBreakPointSite) Type() StopPointType {
	return site.StopPointType
}

func (site *SoftwareBreakPointSite) Address() VirtualAddress {
	return site.address
}

func (site *SoftwareBreakPointSite) IsEnabled() bool {
	return site.isEnabled
}

func (site *SoftwareBreakPointSite) Enable() error {
	if site.isEnabled {
		return nil
	}

	originalData, err := site.swapData(int3Instruction)
	if err != nil {
		return fmt.Errorf("failed to enable software break point site: %w", err)
	}

	site.isEnabled = true
	site.originalData = originalData
	return nil
}

func (site *SoftwareBreakPointSite) Disable() error {
	if !site.isEnabled {
		return nil
	}

	_, err := site.swapData(site.originalData)
	if err != nil {
		return fmt.Errorf("failed to disable software break point site: %w", err)
	}

	site.isEnabled = false
	return nil
}

func (site *SoftwareBreakPointSite) swapData(newData byte) (byte, error) {
	buffer := make([]byte, 1)

	count, err := site.debugger.ReadFromVirtualMemory(site.address, buffer)
	if err != nil {
		return 0, err
	} else if count != 1 {
		return 0, fmt.Errorf(
			"failed to read from memory at %s. "+
				"incorrect number of bytes read (%d != 1)",
			site.address,
			count)
	}

	originalData := buffer[0]
	buffer[0] = newData

	count, err = site.debugger.WriteToVirtualMemory(site.address, buffer)
	if err != nil {
		return 0, err
	} else if count != 1 {
		return 0, fmt.Errorf(
			"failed to write to memory at %s. "+
				"incorrect number of bytes written (%d != 1)",
			site.address,
			count)
	}

	return originalData, nil
}

func (site *SoftwareBreakPointSite) ReplaceStopPointBytes(
	startAddr VirtualAddress,
	memorySlice []byte,
) {
	if !site.isEnabled {
		return
	}

	endAddr := startAddr + VirtualAddress(len(memorySlice))
	if startAddr <= site.address && site.address < endAddr {
		memorySlice[int(site.address-startAddr)] = site.originalData
	}
}

func (site *SoftwareBreakPointSite) deallocate() error {
	return site.Disable()
}
