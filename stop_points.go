package bad

import (
	"fmt"
	"sort"

	"github.com/pattyshack/bad/ptrace"
)

const (
	int3Instruction = byte(0xcc)
)

var (
	ErrInvalidStopPointAddress = fmt.Errorf("invalid stop point address")
)

type StopPoint interface {
	Address() VirtualAddress

	IsEnabled() bool

	Enable() error
	Disable() error

	// If an enabled stop point is in the range
	//    [startAddr, startAddr + len(memorySlice))
	// replace the stop point bytes with the original data bytes in the
	// memorySlice.
	ReplaceStopPointBytes(startAddr VirtualAddress, memorySlice []byte)
}

type StopPointFactory interface {
	Type() string

	newStopPoint(VirtualAddress) StopPoint
}

type StopPointSet struct {
	StopPointFactory

	stopPoints map[VirtualAddress]StopPoint
}

func newStopPointSet(factory StopPointFactory) *StopPointSet {
	return &StopPointSet{
		StopPointFactory: factory,
		stopPoints:       map[VirtualAddress]StopPoint{},
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

func (set *StopPointSet) Set(addr VirtualAddress) (StopPoint, error) {
	_, ok := set.stopPoints[addr]
	if ok {
		return nil, fmt.Errorf(
			"%w. %s at %s already exist",
			ErrInvalidStopPointAddress,
			set.Type(),
			addr)
	}

	stopPoint := set.newStopPoint(addr)

	err := stopPoint.Enable()
	if err != nil {
		return nil, fmt.Errorf("failed to set %s at %s: %w", set.Type(), addr, err)
	}

	set.stopPoints[addr] = stopPoint

	return stopPoint, nil
}

func (set *StopPointSet) Remove(addr VirtualAddress) error {
	stopPoint, ok := set.stopPoints[addr]
	if !ok {
		return fmt.Errorf(
			"%w. no %s found at %s",
			ErrInvalidStopPointAddress,
			set.Type(),
			addr)
	}

	err := stopPoint.Disable()
	if err != nil {
		return fmt.Errorf("cannot remove %s at %s: %w", set.Type(), addr, err)
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

type BreakPointSite struct {
	tracer *ptrace.Tracer

	address      VirtualAddress
	isEnabled    bool
	originalData byte
}

func (site *BreakPointSite) Address() VirtualAddress {
	return site.address
}

func (site *BreakPointSite) IsEnabled() bool {
	return site.isEnabled
}

func (site *BreakPointSite) Enable() error {
	if site.isEnabled {
		return nil
	}

	originalData, err := site.swapData(int3Instruction)
	if err != nil {
		return fmt.Errorf("failed to enable break point site: %w", err)
	}

	site.isEnabled = true
	site.originalData = originalData
	return nil
}

func (site *BreakPointSite) Disable() error {
	if !site.isEnabled {
		return nil
	}

	_, err := site.swapData(site.originalData)
	if err != nil {
		return fmt.Errorf("failed to disable break point site: %w", err)
	}

	site.isEnabled = false
	return nil
}

func (site *BreakPointSite) swapData(newData byte) (byte, error) {
	buffer := make([]byte, 1)

	count, err := site.tracer.PeekData(uintptr(site.address), buffer)
	if err != nil {
		return 0, err
	} else if count != 1 {
		return 0, fmt.Errorf(
			"failed to peek data at %s. incorrect number of bytes peeked (%d != 1)",
			site.address,
			count)
	}

	originalData := buffer[0]
	buffer[0] = newData

	count, err = site.tracer.PokeData(uintptr(site.address), buffer)
	if err != nil {
		return 0, err
	} else if count != 1 {
		return 0, fmt.Errorf(
			"failed to poke data at %s. incorrect number of bytes poked (%d != 1)",
			site.address,
			count)
	}

	return originalData, nil
}

func (site *BreakPointSite) ReplaceStopPointBytes(
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

type breakPointSiteFactory struct {
	tracer *ptrace.Tracer
}

func (breakPointSiteFactory) Type() string {
	return "BreakPointSite"
}

func (factory breakPointSiteFactory) newStopPoint(
	addr VirtualAddress,
) StopPoint {
	return &BreakPointSite{
		tracer:       factory.tracer,
		address:      addr,
		isEnabled:    false,
		originalData: 0,
	}
}

func NewBreakPointSites(tracer *ptrace.Tracer) *StopPointSet {
	return newStopPointSet(breakPointSiteFactory{tracer})
}
