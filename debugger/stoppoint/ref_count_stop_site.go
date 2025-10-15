package stoppoint

import (
	"fmt"

	. "github.com/pattyshack/bad/debugger/common"
)

type refCountStopSite struct {
	pool     *refCountStopSitePool
	refCount int
	StopSite
}

func (site *refCountStopSite) RefCount() int {
	return site.refCount
}

func (site *refCountStopSite) Deallocate() error {
	return site.pool.deallocate(site)
}

type refCountStopSitePool struct {
	software StopSitePool
	hardware StopSitePool

	allocated map[StopSiteKey]*refCountStopSite
}

func NewStopSitePool(
	process Process,
) StopSitePool {
	return &refCountStopSitePool{
		software:  newSoftwareStopSitePool(process.Memory()),
		hardware:  newHardwareStopSitePool(process),
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
		pool:     pool,
		refCount: 1,
		StopSite: base,
	}
	pool.allocated[key] = site
	return site, nil
}

func (pool *refCountStopSitePool) deallocate(
	site *refCountStopSite,
) error {
	site.refCount -= 1

	if site.refCount == 0 {
		delete(pool.allocated, site.Key())
		return site.StopSite.Deallocate()
	} else if site.refCount < 0 {
		return fmt.Errorf("%s already deallocated", site.Key())
	}

	return nil
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

func (pool *refCountStopSitePool) RefreshSites() error {
	err := pool.software.RefreshSites()
	if err != nil {
		return err
	}

	return pool.hardware.RefreshSites()
}
