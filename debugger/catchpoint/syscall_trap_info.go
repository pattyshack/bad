package catchpoint

import (
	"fmt"

	"github.com/pattyshack/bad/debugger/registers"
)

type SyscallTrapInfo struct {
	IsEntry bool

	Id   SyscallId
	Args [6]uint64
	Ret  uint64
}

func NewSyscallTrapEntryInfo(registerState registers.State) *SyscallTrapInfo {
	sysNum := int(registerState.Value(registers.SyscallNum).ToUint32())
	id, _ := SyscallIdByNumber(sysNum)

	info := &SyscallTrapInfo{
		IsEntry: true,
		Id:      id,
	}

	for idx, reg := range registers.SyscallArgs {
		info.Args[idx] = registerState.Value(reg).ToUint64()
	}

	return info
}

func NewSyscallTrapExitInfo(registerState registers.State) *SyscallTrapInfo {
	sysNum := int(registerState.Value(registers.SyscallNum).ToUint32())
	id, _ := SyscallIdByNumber(sysNum)

	return &SyscallTrapInfo{
		IsEntry: false,
		Id:      id,
		Ret:     registerState.Value(registers.SyscallRet).ToUint64(),
	}
}

func (info SyscallTrapInfo) String() string {
	result := "syscall " + info.Id.Name
	if info.IsEntry {
		result += " entry:"
		for _, arg := range info.Args {
			result += fmt.Sprintf(" 0x%x", arg)
		}
	} else {
		result += fmt.Sprintf(" returned: 0x%x", info.Ret)
	}
	return result
}
