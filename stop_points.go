package bad

import (
	"fmt"
	"sort"
)

var (
	ErrInvalidArgument = fmt.Errorf("invalid argument")
)

type StopPointKind string

const (
	SoftwareKind = StopPointKind("software")
	HardwareKind = StopPointKind("hardware")
)

type StopPointMode string

const (
	WriteMode     = StopPointMode("write")
	ReadWriteMode = StopPointMode("read/write")
	ExecuteMode   = StopPointMode("execute")
)

type StopPointType struct {
	Kind      StopPointKind
	Mode      StopPointMode
	WatchSize int // 1, 2, 4, 8
}

func (t StopPointType) String() string {
	stopPoint := ""
	if t.Mode == ExecuteMode && t.WatchSize == 1 {
		stopPoint = " break point site"
	} else {
		stopPoint = " " + string(t.Mode) + " watch point"
	}

	size := ""
	if t.WatchSize > 1 {
		size = fmt.Sprintf(" (size=%d)", t.WatchSize)
	}

	return string(t.Kind) + stopPoint + size
}

func (t StopPointType) Validate(address VirtualAddress) error {
	switch t.Kind {
	case HardwareKind:
		// do nothing
	case SoftwareKind:
		if t.Mode != ExecuteMode {
			return fmt.Errorf(
				"%w. invalid software break point site mode (%s)",
				ErrInvalidArgument,
				t.Mode)
		}

		if t.WatchSize != 1 {
			return fmt.Errorf(
				"%w. invalid software break point site watch size (%d)",
				ErrInvalidArgument,
				t.WatchSize)
		}
	default:
		return fmt.Errorf(
			"%w. invalid stop point kind (%s)",
			ErrInvalidArgument,
			t.Kind)
	}

	switch t.Mode {
	case WriteMode, ReadWriteMode, ExecuteMode:
		// do nothing
	default:
		return fmt.Errorf(
			"%w. invalid stop point mode (%s)",
			ErrInvalidArgument,
			t.Mode)
	}

	switch t.WatchSize {
	case 1, 2, 4, 8:
		if uint64(address)%uint64(t.WatchSize) != 0 {
			return fmt.Errorf(
				"%w. address (0x%x) not aligned with watch size (%d)",
				ErrInvalidArgument,
				address,
				t.WatchSize)
		}
	default:
		return fmt.Errorf(
			"%w. invalid watch size (%d)",
			ErrInvalidArgument,
			t.WatchSize)
	}

	return nil
}

type StopPointOptions struct {
	Type StopPointType
}

type StopPoint interface {
	Type() StopPointType

	Address() VirtualAddress

	IsEnabled() bool

	Enable() error
	Disable() error

	// If an enabled stop point is in the range
	//    [startAddr, startAddr + len(memorySlice))
	// replace the stop point bytes with the original data bytes in the
	// memorySlice.
	ReplaceStopPointBytes(startAddr VirtualAddress, memorySlice []byte)

	// Called by stop point set on Remove.  deallocate must disable the stop
	// point and perform necessary cleanup.
	deallocate() error
}

type StopPointAllocator interface {
	SetDebugger(debugger *Debugger)

	Allocate(address VirtualAddress, options StopPointOptions) (StopPoint, error)
}

type stopPointAllocator struct {
	software softwareBreakPointSiteAllocator
	hardware hardwareStopPointAllocator
}

func NewStopPointAllocator() StopPointAllocator {
	return &stopPointAllocator{}
}

func (allocator *stopPointAllocator) SetDebugger(debugger *Debugger) {
	allocator.software.SetDebugger(debugger)
	allocator.hardware.SetDebugger(debugger)
}

func (allocator *stopPointAllocator) Allocate(
	address VirtualAddress,
	options StopPointOptions,
) (
	StopPoint,
	error,
) {
	if options.Type.Kind == SoftwareKind {
		return allocator.software.Allocate(address, options)
	} else {
		return allocator.hardware.Allocate(address, options)
	}
}

type breakPointSiteAllocator struct {
	base StopPointAllocator
}

func (allocator breakPointSiteAllocator) SetDebugger(debugger *Debugger) {
	allocator.base.SetDebugger(debugger)
}

func (allocator breakPointSiteAllocator) Allocate(
	address VirtualAddress,
	options StopPointOptions,
) (
	StopPoint,
	error,
) {
	if options.Type.Mode != ExecuteMode {
		return nil, fmt.Errorf(
			"%w. invalid break point site mode (%s)",
			ErrInvalidArgument,
			options.Type.Mode)
	}

	if options.Type.WatchSize != 1 {
		return nil, fmt.Errorf(
			"%w. invalid break point site watch size (%d)",
			ErrInvalidArgument,
			options.Type.WatchSize)
	}

	return allocator.base.Allocate(address, options)
}

type watchPointAllocator struct {
	base StopPointAllocator
}

func (allocator watchPointAllocator) SetDebugger(debugger *Debugger) {
	allocator.base.SetDebugger(debugger)
}

func (allocator watchPointAllocator) Allocate(
	address VirtualAddress,
	options StopPointOptions,
) (
	StopPoint,
	error,
) {
	if options.Type.Kind != HardwareKind {
		return nil, fmt.Errorf(
			"%w. invalid watch point kind (%s)",
			ErrInvalidArgument,
			options.Type.Kind)
	}

	return allocator.base.Allocate(address, options)
}

type StopPointSet struct {
	allocator StopPointAllocator

	stopPoints map[VirtualAddress]StopPoint
}

func NewBreakPointSites(allocator StopPointAllocator) *StopPointSet {
	return &StopPointSet{
		allocator: breakPointSiteAllocator{
			base: allocator,
		},
		stopPoints: map[VirtualAddress]StopPoint{},
	}
}

func NewWatchPoints(allocator StopPointAllocator) *StopPointSet {
	return &StopPointSet{
		allocator: watchPointAllocator{
			base: allocator,
		},
		stopPoints: map[VirtualAddress]StopPoint{},
	}
}

func (set *StopPointSet) List() []StopPoint {
	result := make([]StopPoint, 0, len(set.stopPoints))
	for _, stopPoint := range set.stopPoints {
		result = append(result, stopPoint)
	}

	sort.Slice(
		result,
		func(i int, j int) bool {
			return result[i].Address() < result[j].Address()
		})
	return result
}

func (set *StopPointSet) Get(addr VirtualAddress) (StopPoint, bool) {
	stopPoint, ok := set.stopPoints[addr]
	return stopPoint, ok
}

func (set *StopPointSet) Set(
	addr VirtualAddress,
	options StopPointOptions,
) (
	StopPoint,
	error,
) {
	_, ok := set.stopPoints[addr]
	if ok {
		return nil, fmt.Errorf(
			"%w. stop point at %s already exist",
			ErrInvalidArgument,
			addr)
	}

	stopPoint, err := set.allocator.Allocate(addr, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create stop point at %s: %w", addr, err)
	}

	err = stopPoint.Enable()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to set %s at %s: %w",
			options.Type,
			addr,
			err)
	}

	set.stopPoints[addr] = stopPoint

	return stopPoint, nil
}

func (set *StopPointSet) Remove(addr VirtualAddress) error {
	stopPoint, ok := set.stopPoints[addr]
	if !ok {
		return fmt.Errorf(
			"%w. no stop point found at %s",
			ErrInvalidArgument,
			addr)
	}

	err := stopPoint.deallocate()
	if err != nil {
		return fmt.Errorf("cannot remove %s at %s: %w", stopPoint.Type(), addr, err)
	}

	delete(set.stopPoints, addr)
	return nil
}

func (set *StopPointSet) ReplaceStopPointBytes(
	startAddr VirtualAddress,
	memorySlice []byte,
) {
	for _, site := range set.stopPoints {
		site.ReplaceStopPointBytes(startAddr, memorySlice)
	}
}
