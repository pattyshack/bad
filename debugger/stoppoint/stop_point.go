package stoppoint

import (
	"fmt"
	"sort"

	. "github.com/pattyshack/bad/debugger/common"
)

type StopPointType struct {
	IsWatchPoint bool // false for break point
	StopSiteType
}

func (t StopPointType) String() string {
	prefix := "software"
	if t.StopSiteType.IsHardware {
		prefix = "hardware"
	}

	if t.IsWatchPoint {
		return fmt.Sprintf("%s %s watch point (size=%d)",
			prefix,
			t.StopSiteType.Mode,
			t.StopSiteType.WatchSize)
	}

	return prefix + " break point"
}

type StopPointSet struct {
	isWatchPoints bool
	siteAllocator StopSiteAllocator

	nextId    int64
	allocated map[int64]*StopPoint
}

func NewWatchPointSet(allocator StopSiteAllocator) *StopPointSet {
	return &StopPointSet{
		isWatchPoints: true,
		siteAllocator: watchSiteAllocator{
			base: allocator,
		},
		nextId:    0,
		allocated: map[int64]*StopPoint{},
	}
}

func NewBreakPointSet(allocator StopSiteAllocator) *StopPointSet {
	return &StopPointSet{
		isWatchPoints: false,
		siteAllocator: breakSiteAllocator{
			base: allocator,
		},
		nextId:    0,
		allocated: map[int64]*StopPoint{},
	}
}

func (set *StopPointSet) IsWatchPoints() bool {
	return set.isWatchPoints
}

func (set *StopPointSet) Set(
	resolver StopSiteResolver,
	siteType StopSiteType,
	enableOnCreation bool,
) (
	*StopPoint,
	error,
) {
	id := set.nextId
	set.nextId += 1

	point := &StopPoint{
		set:      set,
		id:       id,
		resolver: resolver,
		pointType: StopPointType{
			IsWatchPoint: set.isWatchPoints,
			StopSiteType: siteType,
		},
		isEnabled: enableOnCreation,
	}

	err := point.ResolveStopSites()
	if err != nil {
		return nil, err
	}

	set.allocated[id] = point
	return point, nil
}

func (set *StopPointSet) Remove(id int64) error {
	point, ok := set.allocated[id]
	if !ok {
		return fmt.Errorf("%w. stop point id (%d) not found", ErrInvalidInput, id)
	}

	for _, site := range point.sites {
		err := site.Deallocate()
		if err != nil {
			return fmt.Errorf(
				"failed to remove %s (id=%d). cannot deallocate %s: %w",
				point.Type(),
				id,
				site.Key(),
				err)
		}
	}

	delete(set.allocated, id)
	return nil
}

func (set *StopPointSet) Get(id int64) (*StopPoint, bool) {
	point, ok := set.allocated[id]
	return point, ok
}

func (set *StopPointSet) List() []*StopPoint {
	result := make([]*StopPoint, 0, len(set.allocated))
	for _, point := range set.allocated {
		result = append(result, point)
	}

	sort.Slice(
		result,
		func(i int, j int) bool { return result[i].id < result[j].id })
	return result
}

type Triggered struct {
	*StopPoint
	StopSite
}

func (set *StopPointSet) Match(
	triggeredKeys map[StopSiteKey]struct{},
) []Triggered {
	if len(triggeredKeys) == 0 {
		return nil
	}

	result := []Triggered{}
	for _, point := range set.allocated {
		for _, site := range point.sites {
			_, ok := triggeredKeys[site.Key()]
			if ok {
				result = append(
					result,
					Triggered{
						StopPoint: point,
						StopSite:  site,
					})
				break
			}
		}
	}

	sort.Slice(
		result,
		func(i int, j int) bool { return result[i].Id() < result[j].Id() })
	return result
}

func (set *StopPointSet) ResolveStopSites() error {
	for _, point := range set.allocated {
		err := point.ResolveStopSites()
		if err != nil {
			return err
		}
	}
	return nil
}

type StopPoint struct {
	set *StopPointSet

	id int64

	resolver  StopSiteResolver
	pointType StopPointType

	isEnabled bool

	sites []StopSite
}

func (point *StopPoint) Id() int64 {
	return point.id
}

func (point *StopPoint) Resolver() StopSiteResolver {
	return point.resolver
}

func (point *StopPoint) Type() StopPointType {
	return point.pointType
}

func (point *StopPoint) IsEnabled() bool {
	return point.isEnabled
}

func (point *StopPoint) Sites() []StopSite {
	return point.sites
}

func (point *StopPoint) Enable() error {
	for _, site := range point.sites {
		err := site.Enable()
		if err != nil {
			return fmt.Errorf(
				"failed to enable %s (id=%d). could not enable %s: %w",
				point.Type(),
				point.Id(),
				site.Key(),
				err)
		}
	}

	point.isEnabled = true
	return nil
}

func (point *StopPoint) Disable() error {
	for _, site := range point.sites {
		err := site.Disable()
		if err != nil {
			return fmt.Errorf(
				"failed to disable %s (id=%d). could not disable %s: %w",
				point.Type(),
				point.Id(),
				site.Key(),
				err)
		}
	}

	point.isEnabled = false
	return nil
}

func (point *StopPoint) ResolveStopSites() error {
	addresses, err := point.resolver.ResolveAddresses()
	if err != nil {
		return fmt.Errorf(
			"failed to resolve %s (id=%d). cannot resolve site addresses: %w",
			point.Type(),
			point.Id(),
			err)
	}

	sorted := VirtualAddresses{}
	entries := map[VirtualAddress]StopSite{}
	for _, addr := range addresses {
		_, ok := entries[addr]
		if !ok {
			sorted = append(sorted, addr)
			entries[addr] = nil
		}
	}

	sort.Sort(sorted)

	for _, site := range point.sites {
		_, ok := entries[site.Address()]
		if ok {
			entries[site.Address()] = site
		} else {
			err := site.Deallocate()
			if err != nil {
				return fmt.Errorf(
					"failed to resolve %s (id=%d). cannot deallocate %s: %w",
					point.Type(),
					point.Id(),
					site.Key(),
					err)
			}
		}
	}

	sites := make([]StopSite, 0, len(sorted))
	for _, addr := range sorted {
		site := entries[addr]
		if site == nil {
			var err error
			site, err = point.set.siteAllocator.Allocate(
				addr,
				point.pointType.StopSiteType)
			if err != nil {
				return fmt.Errorf(
					"failed to resolve %s (id=%d). cannot allocate %s at %s: %w",
					point.Type(),
					point.Id(),
					point.pointType.StopSiteType,
					addr,
					err)
			}

			if point.isEnabled {
				err := site.Enable()
				if err != nil {
					return fmt.Errorf(
						"failed to resolve %s (id=%d). cannot enable %s: %w",
						point.Type(),
						point.Id(),
						site.Key(),
						err)
				}
			}
		}

		sites = append(sites, site)
	}

	point.sites = sites
	return nil
}
