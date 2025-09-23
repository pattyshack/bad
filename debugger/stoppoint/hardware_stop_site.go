package stoppoint

import (
	"fmt"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/debugger/registers"
)

const (
	debugStatusRegister  = "dr6"
	debugControlRegister = "dr7"
)

type hardwareStopSitePool struct {
	registers *registers.Registers
	memory    *memory.VirtualMemory
	stopSites [4]*hardwareStopSite
}

func newHardwareStopSitePool(
	registers *registers.Registers,
	memory *memory.VirtualMemory,
) StopSitePool {
	return &hardwareStopSitePool{
		registers: registers,
		memory:    memory,
	}
}

func (pool *hardwareStopSitePool) Allocate(
	address VirtualAddress,
	siteType StopSiteType,
) (
	StopSite,
	error,
) {
	if !siteType.IsHardware {
		return nil, fmt.Errorf(
			"%w. cannot allocate software stop site",
			ErrInvalidArgument)
	}

	err := siteType.Validate(address)
	if err != nil {
		return nil, err
	}

	for idx, site := range pool.stopSites {
		if site == nil {
			site = &hardwareStopSite{
				pool:      pool,
				siteType:  siteType,
				address:   address,
				isEnabled: false,
			}
			pool.stopSites[idx] = site

			err = pool.updateStopSiteData(site)
			if err != nil {
				return nil, fmt.Errorf("failed to allocate hardware stop site: %w", err)
			}

			return site, nil
		}
	}

	return nil, fmt.Errorf(
		"%w. all available hardware stop sites occupied",
		ErrInvalidArgument)
}

func (pool *hardwareStopSitePool) deallocate(
	site *hardwareStopSite,
) error {
	for idx, allocated := range pool.stopSites {
		if site == allocated {
			pool.stopSites[idx] = nil
		}
	}

	return pool.updateDebugRegisters()
}

func (pool *hardwareStopSitePool) controlBytes() uint64 {
	// Control bits (least to most significant:
	//   0:     dr0 local stop site enabled
	//   1:     dr0 global stop site enabled (not applicable in linux)
	//   2:     dr1 local stop site enabled
	//   3:     dr1 global stop site enabled (not applicable in linux)
	//   4:     dr2 local stop site enabled
	//   5:     dr2 global stop site enabled (not applicable in linux)
	//   6:     dr3 local stop site enabled
	//   7:     dr3 global stop site enabled (not applicable in linux)
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

	for idx, site := range pool.stopSites {
		if site == nil || !site.isEnabled {
			continue
		}

		enabledBytes |= 0b01 << (2 * idx)

		// NOTE: I/0 read and writes mode (0b10) is not supported
		switch site.siteType.Mode {
		case ExecuteMode:
			// do nothing.  The control bits are 0b00 for execute
		case WriteMode:
			conditionBytes |= 0b01 << (4 * idx)
		case ReadWriteMode:
			conditionBytes |= 0b11 << (4 * idx)
		default:
			panic("should never happen")
		}

		switch site.siteType.WatchSize {
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

func (pool *hardwareStopSitePool) updateDebugRegisters() error {
	state, err := pool.registers.GetState()
	if err != nil {
		return fmt.Errorf("failed to update hardware stop sites: %w", err)
	}

	reg, ok := registers.ByName(debugControlRegister)
	if !ok {
		panic("should never happen")
	}

	state, err = state.WithValue(reg, registers.U64(pool.controlBytes()))
	if err != nil {
		return fmt.Errorf("failed to update hardware stop sites: %w", err)
	}

	for idx, site := range pool.stopSites {
		reg, ok := registers.ByName(fmt.Sprintf("dr%d", idx))
		if !ok {
			panic("should never happen")
		}

		addr := uint64(0)
		if site != nil {
			addr = uint64(site.address)
		}

		state, err = state.WithValue(reg, registers.U64(addr))
		if err != nil {
			fmt.Errorf("failed to update hardware stop sites: %w", err)
		}
	}

	err = pool.registers.SetState(state)
	if err != nil {
		return fmt.Errorf("failed to update hardware stop sites: %w", err)
	}

	return nil
}

func (pool *hardwareStopSitePool) updateStopSiteData(
	site *hardwareStopSite,
) error {
	content := make([]byte, site.siteType.WatchSize)
	n, err := pool.memory.Read(site.address, content)
	if err != nil {
		return fmt.Errorf("failed to update hardware stop site data: %w", err)
	}

	if n != site.siteType.WatchSize {
		return fmt.Errorf(
			"failed to update hardware stop site data. "+
				"incorrect number of bytes read (%d != %d)",
			site.siteType.WatchSize,
			n)
	}

	site.previousData = site.data
	site.data = content
	return nil
}

func (pool *hardwareStopSitePool) GetEnabledAt(
	addr VirtualAddress,
) StopSites {
	result := []StopSite{}
	for _, site := range pool.stopSites {
		if site != nil && site.Address() == addr && site.IsEnabled() {
			result = append(result, site)
		}
	}
	return result
}

func (hardwareStopSitePool) ReplaceStopSiteBytes(
	startAddr VirtualAddress,
	memorySlice []byte,
) {
}

func (pool *hardwareStopSitePool) ListTriggered(
	pc VirtualAddress,
	kind TrapKind,
) (
	VirtualAddress,
	map[StopSiteKey]struct{},
	error,
) {
	if kind != HardwareTrap {
		return pc, nil, nil
	}

	state, err := pool.registers.GetState()
	if err != nil {
		return pc, nil, fmt.Errorf("failed to list triggered stop sites: %w", err)
	}

	reg, ok := registers.ByName(debugStatusRegister)
	if !ok {
		panic("should never happen")
	}

	status := state.Value(reg).ToUint64()
	triggered := map[StopSiteKey]struct{}{}
	for idx, site := range pool.stopSites {
		if status&uint64(1<<idx) > 0 {
			if site == nil {
				panic("should never happen")
			}
			triggered[site.Key()] = struct{}{}

			err = pool.updateStopSiteData(site)
			if err != nil {
				return pc, nil, fmt.Errorf(
					"failed to list triggered stop sites: %w",
					err)
			}
		}
	}

	return pc, triggered, nil
}

type hardwareStopSite struct {
	pool *hardwareStopSitePool

	siteType StopSiteType

	address   VirtualAddress
	isEnabled bool

	previousData []byte
	data         []byte
}

func (site *hardwareStopSite) Type() StopSiteType {
	return site.siteType
}

func (site *hardwareStopSite) Address() VirtualAddress {
	return site.address
}

func (site *hardwareStopSite) Key() StopSiteKey {
	return StopSiteKey{
		VirtualAddress: site.address,
		StopSiteType:   site.siteType,
	}
}

func (hardwareStopSite) RefCount() int {
	return 1
}

func (site *hardwareStopSite) Deallocate() error {
	return site.pool.deallocate(site)
}

func (site *hardwareStopSite) IsEnabled() bool {
	return site.isEnabled
}

func (site *hardwareStopSite) Enable() error {
	if site.isEnabled {
		return nil
	}

	site.isEnabled = true
	err := site.pool.updateDebugRegisters()
	if err != nil {
		return fmt.Errorf(
			"failed to enable %s at %s: %w",
			site.siteType,
			site.address,
			err)
	}

	return nil
}

func (site *hardwareStopSite) Disable() error {
	if !site.isEnabled {
		return nil
	}

	site.isEnabled = false
	err := site.pool.updateDebugRegisters()
	if err != nil {
		return fmt.Errorf("failed to disable %s at %s: %w",
			site.siteType,
			site.address,
			err)
	}

	return nil
}

func (hardwareStopSite) ReplaceStopSiteBytes(
	startAddr VirtualAddress,
	memorySlice []byte,
) {
	// do nothing
}

func (site *hardwareStopSite) PreviousData() []byte {
	return site.previousData
}

func (site *hardwareStopSite) Data() []byte {
	return site.data
}
