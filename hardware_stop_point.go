package bad

import (
	"fmt"
)

const (
	debugStatusRegister  = "dr6"
	debugControlRegister = "dr7"
)

func HardwareWatchPointOptions(
	mode StopPointMode,
	watchSize int,
) StopPointOptions {
	return StopPointOptions{
		Type: StopPointType{
			IsBreakPoint: false,
			Kind:         HardwareKind,
			Mode:         mode,
			WatchSize:    watchSize,
		},
	}
}

func HardwareBreakPointSiteOptions() StopPointOptions {
	return StopPointOptions{
		Type: StopPointType{
			IsBreakPoint: true,
			Kind:         HardwareKind,
			Mode:         ExecuteMode,
			WatchSize:    1,
		},
	}
}

type hardwareStopPointAllocator struct {
	debugger   *Debugger
	stopPoints [4]*HardwareStopPoint
}

func newHardwareStopPointAllocator() *hardwareStopPointAllocator {
	return &hardwareStopPointAllocator{}
}

func (allocator *hardwareStopPointAllocator) SetDebugger(debugger *Debugger) {
	allocator.debugger = debugger
}

func (allocator *hardwareStopPointAllocator) Allocate(
	address VirtualAddress,
	options StopPointOptions,
) (
	StopPoint,
	error,
) {
	if options.Type.Kind != HardwareKind {
		return nil, fmt.Errorf(
			"%w. invalid stop point kind (%s)",
			ErrInvalidArgument,
			options.Type.Kind)
	}

	err := options.Type.Validate(address)
	if err != nil {
		return nil, err
	}

	for idx, sp := range allocator.stopPoints {
		if sp == nil {
			sp = &HardwareStopPoint{
				allocator:     allocator,
				StopPointType: options.Type,
				address:       address,
				isEnabled:     false,
			}
			allocator.stopPoints[idx] = sp

			err = allocator.updateStopPointData(sp)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to allocate hardware stop point: %w",
					err)
			}

			return sp, nil
		}
	}

	return nil, fmt.Errorf(
		"%w. all available hardware stop points occupied",
		ErrInvalidArgument)
}

func (allocator *hardwareStopPointAllocator) deallocate(
	sp *HardwareStopPoint,
) error {
	for idx, allocated := range allocator.stopPoints {
		if sp == allocated {
			allocator.stopPoints[idx] = nil
		}
	}

	return allocator.updateDebugRegisters()
}

func (allocator *hardwareStopPointAllocator) controlBytes() uint64 {
	// Control bits (least to most significant:
	//   0:     dr0 local breakpoint enabled
	//   1:     dr0 global breakpoint enabled (not applicable in linux)
	//   2:     dr1 local breakpoint enabled
	//   3:     dr1 global breakpoint enabled (not applicable in linux)
	//   4:     dr2 local breakpoint enabled
	//   5:     dr2 global breakpoint enabled (not applicable in linux)
	//   6:     dr3 local breakpoint enabled
	//   7:     dr3 global breakpoint enabled (not applicable in linux)
	//   8-15:  reserved / not applicable
	//   16-17: dr0 conditions
	//   18-19: dr0 watch size
	//   20-21: dr1 conditions
	//   22-23: dr1 watch size
	//   24-25: dr2 conditions
	//   26-27: dr2 watch size
	//   28-29: dr3 conditions
	//   30-31: dr3 watch size
	//
	// Condition bits:
	//   0b00: instruction execution only
	//   0b01: data write only
	//   0b10: I/O reads and writes (not supported by linux)
	//   0b11: data reads and writes
	//
	// Watch size bits:
	//   0b00: 1 byte
	//   0b01: 2 bytes
	//   0b10: 8 bytes
	//   0b11: 4 bytes

	enabledBytes := uint64(0)
	conditionBytes := uint64(0)
	watchSizeBytes := uint64(0)

	for idx, sp := range allocator.stopPoints {
		if sp == nil || !sp.isEnabled {
			continue
		}

		enabledBytes |= 0b01 << (2 * idx)

		// NOTE: I/0 read and writes mode (0b10) is not supported
		switch sp.Mode {
		case ExecuteMode:
			// do nothing.  The control bits are 0b00 for execute
		case WriteMode:
			conditionBytes |= 0b01 << (4 * idx)
		case ReadWriteMode:
			conditionBytes |= 0b11 << (4 * idx)
		default:
			panic("should never happen")
		}

		switch sp.WatchSize {
		case 1:
			// do nothing.  The size bits are 0b00 for 1 byte
		case 2:
			watchSizeBytes |= 0b01 << (4 * idx)
		case 4:
			watchSizeBytes |= 0b11 << (4 * idx)
		case 8:
			watchSizeBytes |= 0b10 << (4 * idx)
		default:
			panic("should never happen")
		}
	}

	conditionBytes <<= 16
	watchSizeBytes <<= 18

	return watchSizeBytes | conditionBytes | enabledBytes
}

func (allocator *hardwareStopPointAllocator) updateDebugRegisters() error {
	state, err := allocator.debugger.GetRegisterState()
	if err != nil {
		return fmt.Errorf("failed to update hardware stop points: %w", err)
	}

	reg, ok := allocator.debugger.RegisterByName(debugControlRegister)
	if !ok {
		panic("should never happen")
	}

	state, err = state.WithValue(reg, Uint64Value(allocator.controlBytes()))
	if err != nil {
		return fmt.Errorf("failed to update hardware stop points: %w", err)
	}

	for idx, sp := range allocator.stopPoints {
		reg, ok := allocator.debugger.RegisterByName(fmt.Sprintf("dr%d", idx))
		if !ok {
			panic("should never happen")
		}

		addr := uint64(0)
		if sp != nil {
			addr = uint64(sp.address)
		}

		state, err = state.WithValue(reg, Uint64Value(addr))
		if err != nil {
			fmt.Errorf("failed to update hardware stop points: %w", err)
		}
	}

	err = allocator.debugger.SetRegisterState(state)
	if err != nil {
		return fmt.Errorf("failed to update hardware stop points: %w", err)
	}

	return nil
}

func (allocator *hardwareStopPointAllocator) updateStopPointData(
	sp *HardwareStopPoint,
) error {
	content := make([]byte, sp.WatchSize)
	n, err := allocator.debugger.ReadFromVirtualMemory(sp.address, content)
	if err != nil {
		return fmt.Errorf("failed to update hardware stop point data: %w", err)
	}

	if n != sp.WatchSize {
		return fmt.Errorf(
			"failed to update hardware stop point data. "+
				"incorrect number of bytes read (%d != %d)",
			sp.WatchSize,
			n)
	}

	sp.previousData = sp.data
	sp.data = content
	return nil
}

func (allocator *hardwareStopPointAllocator) ListTriggered() (
	[]StopPoint,
	error,
) {
	state, err := allocator.debugger.GetRegisterState()
	if err != nil {
		return nil, fmt.Errorf("failed to list triggered stop points: %w", err)
	}

	reg, ok := allocator.debugger.RegisterByName(debugStatusRegister)
	if !ok {
		panic("should never happen")
	}

	status := state.Value(reg).ToUint64()
	triggered := []StopPoint{}
	for idx, sp := range allocator.stopPoints {
		if status&uint64(1<<idx) > 0 {
			if sp == nil {
				panic("should never happen")
			}
			triggered = append(triggered, sp)

			err = allocator.updateStopPointData(sp)
			if err != nil {
				return nil, fmt.Errorf("failed to list triggered stop points: %w", err)
			}
		}
	}

	return triggered, nil
}

type HardwareStopPoint struct {
	allocator *hardwareStopPointAllocator

	StopPointType

	address   VirtualAddress
	isEnabled bool

	previousData []byte
	data         []byte
}

func (sp *HardwareStopPoint) Type() StopPointType {
	return sp.StopPointType
}

func (sp *HardwareStopPoint) Address() VirtualAddress {
	return sp.address
}

func (sp *HardwareStopPoint) IsEnabled() bool {
	return sp.isEnabled
}

func (sp *HardwareStopPoint) Enable() error {
	if sp.isEnabled {
		return nil
	}

	sp.isEnabled = true
	err := sp.allocator.updateDebugRegisters()
	if err != nil {
		return fmt.Errorf(
			"failed to enable %s at %s: %w",
			sp.StopPointType,
			sp.address,
			err)
	}

	return nil
}

func (sp *HardwareStopPoint) Disable() error {
	if !sp.isEnabled {
		return nil
	}

	sp.isEnabled = false
	err := sp.allocator.updateDebugRegisters()
	if err != nil {
		return fmt.Errorf(
			"failed to disable %s at %s: %w",
			sp.StopPointType,
			sp.address,
			err)
	}

	return nil
}

func (sp *HardwareStopPoint) deallocate() error {
	return sp.allocator.deallocate(sp)
}

func (HardwareStopPoint) ReplaceStopPointBytes(
	startAddr VirtualAddress,
	memorySlice []byte,
) {
	// do nothing
}

func (sp *HardwareStopPoint) PreviousData() []byte {
	return sp.previousData
}

func (sp *HardwareStopPoint) Data() []byte {
	return sp.data
}
