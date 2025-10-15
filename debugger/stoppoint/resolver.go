package stoppoint

import (
	"fmt"
	"sort"

	. "github.com/pattyshack/bad/debugger/common"
	"github.com/pattyshack/bad/debugger/loadedelves"
	"github.com/pattyshack/bad/dwarf"
)

type StopSiteResolverFactory struct {
	loadedElves *loadedelves.Files
}

func NewStopSiteResolverFactory(
	files *loadedelves.Files,
) StopSiteResolverFactory {
	return StopSiteResolverFactory{
		loadedElves: files,
	}
}

func (StopSiteResolverFactory) NewAddressResolver(
	addresses ...VirtualAddress,
) StopSiteResolver {
	sorted := VirtualAddresses{}
	entries := map[VirtualAddress]struct{}{}
	for _, addr := range addresses {
		_, ok := entries[addr]
		if !ok {
			sorted = append(sorted, addr)
			entries[addr] = struct{}{}
		}
	}

	sort.Sort(sorted)

	return &AddressStopSiteResolver{
		Addresses: sorted,
	}
}

func (factory StopSiteResolverFactory) NewLineResolver(
	path string,
	line int,
) StopSiteResolver {
	return &LineStopSiteResolver{
		LoadedElves: factory.loadedElves,
		Path:        path,
		Line:        line,
	}
}

func (factory StopSiteResolverFactory) NewFunctionResolver(
	name string,
) StopSiteResolver {
	return &FunctionStopSiteResolver{
		LoadedElves: factory.loadedElves,
		Name:        name,
	}
}

type StopSiteResolver interface {
	String() string
	ResolveAddresses() (VirtualAddresses, error)
}

type AddressStopSiteResolver struct {
	Addresses VirtualAddresses
}

func (resolver *AddressStopSiteResolver) String() string {
	return fmt.Sprintf("addresses@%v", resolver.Addresses)
}

func (resolver *AddressStopSiteResolver) ResolveAddresses() (
	VirtualAddresses,
	error,
) {
	return resolver.Addresses, nil
}

type FunctionStopSiteResolver struct {
	LoadedElves *loadedelves.Files
	Name        string
}

func (resolver *FunctionStopSiteResolver) String() string {
	return fmt.Sprintf("function@%s", resolver.Name)
}

func (resolver *FunctionStopSiteResolver) ResolveAddresses() (
	VirtualAddresses,
	error,
) {
	result, err := resolver.resolveAddresses()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve addresses for %s: %w",
			resolver,
			err)
	}

	return result, nil
}

func (resolver *FunctionStopSiteResolver) resolveAddresses() (
	VirtualAddresses,
	error,
) {
	prologueBodies := map[VirtualAddress]VirtualAddress{}

	funcDefs, err := resolver.LoadedElves.FunctionDefinitionEntriesWithName(
		resolver.Name)
	if err != nil {
		return nil, err
	}

	for _, funcDef := range funcDefs {
		addressRanges, err := funcDef.AddressRanges()
		if err != nil {
			return nil, err
		}

		if len(addressRanges) == 0 {
			continue
		}

		lowPC, err := resolver.LoadedElves.ToVirtualAddress(
			funcDef.File.File,
			addressRanges[0].Low)
		if err != nil {
			return nil, err
		}

		if funcDef.Tag == dwarf.DW_TAG_inlined_subroutine {
			// Inlined function have no prologue.
			prologueBodies[lowPC] = lowPC
		} else {
			// Extract prologue / body address from dwarf whenever possible
			prologue, err := resolver.LoadedElves.LineEntryAt(lowPC)
			if err != nil {
				return nil, err
			}
			if prologue == nil {
				continue
			}

			body, err := prologue.Next()
			if err != nil {
				return nil, err
			}
			if body == nil {
				return nil, fmt.Errorf("body line entry not found")
			}

			prologueAddr, err := resolver.LoadedElves.LineEntryToVirtualAddress(
				prologue)
			if err != nil {
				return nil, err
			}

			bodyAddr, err := resolver.LoadedElves.LineEntryToVirtualAddress(body)
			if err != nil {
				return nil, err
			}

			prologueBodies[prologueAddr] = bodyAddr
		}
	}

	// Fallback to elf symbol for prologue address
	for _, symbol := range resolver.LoadedElves.SymbolsByName(resolver.Name) {
		prologueAddr, err := resolver.LoadedElves.SymbolToVirtualAddress(symbol)
		if err != nil {
			return nil, err
		}

		_, ok := prologueBodies[prologueAddr]
		if ok {
			continue
		}
		prologueBodies[prologueAddr] = prologueAddr
	}

	set := map[VirtualAddress]struct{}{}
	addresses := VirtualAddresses{}
	for _, body := range prologueBodies {
		_, ok := set[body]
		if ok {
			continue
		}
		set[body] = struct{}{}
		addresses = append(addresses, body)
	}

	return addresses, nil
}

type LineStopSiteResolver struct {
	LoadedElves *loadedelves.Files
	Path        string
	Line        int
}

func (resolver *LineStopSiteResolver) String() string {
	return fmt.Sprintf("line@%s:%d", resolver.Path, resolver.Line)
}

func (resolver *LineStopSiteResolver) ResolveAddresses() (
	VirtualAddresses,
	error,
) {
	result, err := resolver.resolveAddresses()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve addresses for %s: %w",
			resolver,
			err)
	}

	return result, nil
}

func (resolver *LineStopSiteResolver) resolveAddresses() (
	VirtualAddresses,
	error,
) {
	lineEntries, err := resolver.LoadedElves.LineEntriesByLine(
		resolver.Path,
		resolver.Line)
	if err != nil {
		return nil, err
	}

	result := VirtualAddresses{}
	for _, lineEntry := range lineEntries {
		lineAddress, err := resolver.LoadedElves.LineEntryToVirtualAddress(lineEntry)
		if err != nil {
			return nil, err
		}

		// NOTE: funcDef is the outer most function entry
		_, funcDef, err := resolver.LoadedElves.
			FunctionDefinitionEntryContainingAddress(lineAddress)
		if err != nil {
			return nil, err
		}
		if funcDef == nil {
			return nil, fmt.Errorf("no function entry associated with line entry")
		}

		addressRanges, err := funcDef.AddressRanges()
		if err != nil {
			return nil, err
		}

		if len(addressRanges) > 0 &&
			addressRanges[0].Low == lineEntry.FileAddress {

			// lineEntry currently points to the outer-most non-inlined function
			// prologue. Advance to the function body's line entry.

			lineEntry, err = lineEntry.Next()
			if err != nil {
				return nil, err
			}
			if lineEntry == nil {
				return nil, fmt.Errorf("body line entry not found")
			}

			lineAddress, err = resolver.LoadedElves.LineEntryToVirtualAddress(
				lineEntry)
			if err != nil {
				return nil, err
			}
		}

		result = append(result, lineAddress)
	}

	return result, nil
}
