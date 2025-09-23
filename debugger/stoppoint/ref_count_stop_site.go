package stoppoint

import (
	"fmt"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/debugger/registers"
)

type refCountStopSite struct {
	refCount int
	StopSite
}

func (site *refCountStopSite) RefCount() int {
	return site.refCount
}

func (site *refCountStopSite) Deallocate() error {
	site.refCount -= 1

	if site.refCount == 0 {
		return site.StopSite.Deallocate()
	} else if site.refCount < 0 {
		return fmt.Errorf("%s already deallocated", site.Key())
	}

	return nil
}

type refCountStopSitePool struct {
	software StopSitePool
	hardware StopSitePool

	allocated map[StopSiteKey]*refCountStopSite
}

func NewStopSitePool(
	registers *registers.Registers,
	memory *memory.VirtualMemory,
) StopSitePool {
	return &refCountStopSitePool{
		software:  newSoftwareStopSitePool(memory),
		hardware:  newHardwareStopSitePool(registers, memory),
		allocated: map[StopSiteKey]*refCountStopSite{},
	}
}

func (pool *refCountStopSitePool) Allocate(
	address VirtualAddress,
	siteType StopSiteType,
) (
	StopSite,
	error,
) {
	key := StopSiteKey{
		VirtualAddress: address,
		StopSiteType:   siteType,
	}

	site, ok := pool.allocated[key]
	if ok {
		site.refCount += 1
		return site, nil
	}

	var base StopSite
	var err error
	if siteType.IsHardware {
		base, err = pool.hardware.Allocate(address, siteType)
	} else {
		base, err = pool.software.Allocate(address, siteType)
	}

	if err != nil {
		return nil, err
	}

	site = &refCountStopSite{
		refCount: 1,
		StopSite: base,
	}
	pool.allocated[key] = site
	return site, nil
}

func (pool *refCountStopSitePool) GetEnabledAt(
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

func (pool *refCountStopSitePool) ReplaceStopSiteBytes(
	startAddr VirtualAddress,
	memorySlice []byte,
) {
	pool.software.ReplaceStopSiteBytes(startAddr, memorySlice)
}

func (pool *refCountStopSitePool) ListTriggered(
	pc VirtualAddress,
	kind TrapKind,
) (
	VirtualAddress,
	map[StopSiteKey]struct{},
	error,
) {
	if kind == HardwareTrap {
		return pool.hardware.ListTriggered(pc, kind)
	}
	return pool.software.ListTriggered(pc, kind)
}
