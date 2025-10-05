package stoppoint

import (
	"fmt"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/memory"
)

const (
	int3Instruction = byte(0xcc)
)

type softwareStopSitePool struct {
	memory *memory.VirtualMemory

	allocated map[VirtualAddress]*softwareStopSite
}

func newSoftwareStopSitePool(
	mem *memory.VirtualMemory,
) StopSitePool {
	return &softwareStopSitePool{
		memory:    mem,
		allocated: map[VirtualAddress]*softwareStopSite{},
	}
}

func (pool *softwareStopSitePool) Allocate(
	address VirtualAddress,
	siteType StopSiteType,
) (
	StopSite,
	error,
) {
	if siteType.IsHardware {
		return nil, fmt.Errorf(
			"%w. cannot allocate hardware stop site",
			ErrInvalidArgument)
	}

	err := siteType.Validate(address)
	if err != nil {
		return nil, err
	}

	_, ok := pool.allocated[address]
	if ok {
		return nil, fmt.Errorf("duplicate software stop site at %s", address)
	}

	site := &softwareStopSite{
		pool:         pool,
		siteType:     siteType,
		address:      address,
		isEnabled:    false,
		originalData: 0,
	}
	pool.allocated[address] = site
	return site, nil
}

func (pool *softwareStopSitePool) deallocate(
	site *softwareStopSite,
) error {
	foundSite := pool.allocated[site.address]
	if foundSite != site {
		return fmt.Errorf(
			"software stop site at %s already deallocated",
			site.address)
	}

	err := site.Disable()
	if err != nil {
		return err
	}

	delete(pool.allocated, site.address)
	return nil
}

func (pool *softwareStopSitePool) ReplaceStopSiteBytes(
	startAddr VirtualAddress,
	memorySlice []byte,
) {
	for _, site := range pool.allocated {
		site.ReplaceStopSiteBytes(startAddr, memorySlice)
	}
}

func (pool *softwareStopSitePool) GetEnabledAt(
	addr VirtualAddress,
) StopSites {
	result := []StopSite{}
	for _, site := range pool.allocated {
		if site.Address() == addr && site.IsEnabled() {
			result = append(result, site)
		}
	}
	return result
}

func (pool *softwareStopSitePool) ListTriggered(
	pc VirtualAddress,
	kind TrapKind,
) (
	VirtualAddress,
	map[StopSiteKey]struct{},
	error,
) {
	if kind != SoftwareTrap {
		return pc, nil, nil
	}

	// NOTE: stopSiteAddress may not be a valid instruction address since x64
	// instruction could span multiple bytes. However, since software stop sites
	// are implemented using int3 (0xcc), we know for sure the address is valid
	// if the current instruction is a stop site.
	stopSiteAddress := pc - 1

	site, ok := pool.allocated[stopSiteAddress]
	if ok && site.IsEnabled() {
		triggered := map[StopSiteKey]struct{}{
			site.Key(): struct{}{},
		}
		return stopSiteAddress, triggered, nil
	}

	return pc, nil, nil
}

func (softwareStopSitePool) RefreshSites() error {
	return nil
}

type softwareStopSite struct {
	pool *softwareStopSitePool

	siteType StopSiteType

	address      VirtualAddress
	isEnabled    bool
	originalData byte
}

func (site *softwareStopSite) Type() StopSiteType {
	return site.siteType
}

func (site *softwareStopSite) Address() VirtualAddress {
	return site.address
}

func (site *softwareStopSite) Key() StopSiteKey {
	return StopSiteKey{
		VirtualAddress: site.address,
		StopSiteType:   site.siteType,
	}
}

func (softwareStopSite) RefCount() int {
	return 1
}

func (site *softwareStopSite) Deallocate() error {
	return site.pool.deallocate(site)
}

func (site *softwareStopSite) IsEnabled() bool {
	return site.isEnabled
}

func (site *softwareStopSite) Enable() error {
	if site.isEnabled {
		return nil
	}

	originalData, err := site.swapData(int3Instruction)
	if err != nil {
		return fmt.Errorf("failed to enable software stop site: %w", err)
	}

	site.isEnabled = true
	site.originalData = originalData
	return nil
}

func (site *softwareStopSite) Disable() error {
	if !site.isEnabled {
		return nil
	}

	_, err := site.swapData(site.originalData)
	if err != nil {
		return fmt.Errorf("failed to disable software stop site: %w", err)
	}

	site.isEnabled = false
	return nil
}

func (site *softwareStopSite) swapData(newData byte) (byte, error) {
	buffer := make([]byte, 1)

	count, err := site.pool.memory.Read(site.address, buffer)
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

	count, err = site.pool.memory.Write(site.address, buffer)
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

func (site *softwareStopSite) ReplaceStopSiteBytes(
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

func (softwareStopSite) PreviousData() []byte {
	return nil
}

func (softwareStopSite) Data() []byte {
	return nil
}
