package stoppoint

import (
	"fmt"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/memory"
	"github.com/pattyshack/bad/debugger/registers"
)

type Process interface {
	AllRegisters() []*registers.Registers
	Memory() *memory.VirtualMemory
}

type StopSiteMode string

const (
	WriteMode     = StopSiteMode("write")
	ReadWriteMode = StopSiteMode("read/write")
	ExecuteMode   = StopSiteMode("execute")
)

type StopSiteType struct {
	IsHardware bool
	Mode       StopSiteMode
	WatchSize  int // 1, 2, 4, 8
}

func NewBreakSiteType(isHardware bool) StopSiteType {
	return StopSiteType{
		IsHardware: isHardware,
		Mode:       ExecuteMode,
		WatchSize:  1,
	}
}

func NewWatchSiteType(mode StopSiteMode, watchSize int) StopSiteType {
	return StopSiteType{
		IsHardware: true,
		Mode:       mode,
		WatchSize:  watchSize,
	}
}

func (t StopSiteType) String() string {
	kind := "software"
	if t.IsHardware {
		kind = "hardware"
	}

	size := ""
	if t.WatchSize != 1 {
		size = fmt.Sprintf(" (size=%d)", t.WatchSize)
	}

	return fmt.Sprintf("%s %s stop site%s", kind, t.Mode, size)
}

func (t StopSiteType) Validate(address VirtualAddress) error {
	switch t.Mode {
	case WriteMode, ReadWriteMode, ExecuteMode:
		// do nothing
	default:
		return fmt.Errorf(
			"%w. invalid hardware stop site mode (%s)",
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

	if !t.IsHardware {
		if t.Mode != ExecuteMode {
			return fmt.Errorf(
				"%w. invalid software stop site mode (%s)",
				ErrInvalidArgument,
				t.Mode)
		}

		if t.WatchSize != 1 {
			return fmt.Errorf(
				"%w. invalid software stop site watch size (%d)",
				ErrInvalidArgument,
				t.WatchSize)
		}
	}

	return nil
}

type StopSiteKey struct {
	VirtualAddress
	StopSiteType
}

func (key StopSiteKey) String() string {
	return fmt.Sprintf("%s at %s", key.StopSiteType, key.VirtualAddress)
}

// A stop site may be associated with multiple break/watch points.
type StopSite interface {
	memory.StopSiteBytes

	Type() StopSiteType

	Address() VirtualAddress

	Key() StopSiteKey

	RefCount() int

	// Deallocate disables the stop site and perform necessary cleanup.
	Deallocate() error

	IsEnabled() bool

	Enable() error
	Disable() error

	// Mainly used by watch point
	PreviousData() []byte
	Data() []byte
}

type StopSites []StopSite

func (sites StopSites) Enable() error {
	for _, site := range sites {
		err := site.Enable()
		if err != nil {
			return fmt.Errorf("cannot enable %s: %w", site.Key(), err)
		}
	}
	return nil
}

func (sites StopSites) Disable() error {
	for _, site := range sites {
		err := site.Disable()
		if err != nil {
			return fmt.Errorf("cannot disable %s: %w", site.Key(), err)
		}
	}
	return nil
}

type StopSiteAllocator interface {
	Allocate(address VirtualAddress, siteType StopSiteType) (StopSite, error)
}

type StopSitePool interface {
	memory.StopSiteBytes

	StopSiteAllocator

	GetEnabledAt(address VirtualAddress) StopSites

	ListTriggered(
		pc VirtualAddress,
		kind TrapKind,
	) (
		VirtualAddress, // real pc
		map[StopSiteKey]struct{},
		error,
	)

	// Called when the debugger finds new threads.
	RefreshSites() error
}

type watchSiteAllocator struct {
	base StopSiteAllocator
}

func (allocator watchSiteAllocator) Allocate(
	address VirtualAddress,
	siteType StopSiteType,
) (
	StopSite,
	error,
) {
	if !siteType.IsHardware {
		return nil, fmt.Errorf(
			"%w. watch point must use hardware stop site",
			ErrInvalidArgument)
	}

	return allocator.base.Allocate(address, siteType)
}

type breakSiteAllocator struct {
	base StopSiteAllocator
}

func (allocator breakSiteAllocator) Allocate(
	address VirtualAddress,
	siteType StopSiteType,
) (
	StopSite,
	error,
) {
	if siteType.Mode != ExecuteMode || siteType.WatchSize != 1 {
		return nil, fmt.Errorf(
			"%w. break point must use execute mode stop site with watch size of 1",
			ErrInvalidArgument)
	}

	return allocator.base.Allocate(address, siteType)
}
