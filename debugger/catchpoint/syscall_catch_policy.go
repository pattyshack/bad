package catchpoint

type catchMode int

const (
	catchNone = catchMode(0)
	catchAll  = catchMode(1)
	catchList = catchMode(2)
)

type SyscallCatchPolicy struct {
	mode catchMode
	ids  []SyscallId
}

func NewSyscallCatchPolicy() *SyscallCatchPolicy {
	return &SyscallCatchPolicy{
		mode: catchNone,
		ids:  nil,
	}
}

func (policy *SyscallCatchPolicy) IsEnabled() bool {
	return policy.mode == catchAll || policy.mode == catchList
}

func (policy *SyscallCatchPolicy) CatchNone() {
	policy.mode = catchNone
	policy.ids = nil
}

func (policy *SyscallCatchPolicy) CatchAll() {
	policy.mode = catchAll
	policy.ids = nil
}

func (policy *SyscallCatchPolicy) CatchList(ids []SyscallId) {
	policy.mode = catchList
	policy.ids = ids
}

func (policy *SyscallCatchPolicy) Matches(id SyscallId) bool {
	if policy.mode == catchAll {
		return true
	}

	for _, policyId := range policy.ids {
		if id == policyId {
			return true
		}
	}

	return false
}

func (policy *SyscallCatchPolicy) String() string {
	switch policy.mode {
	case catchNone:
		return "catch no syscall"
	case catchAll:
		return "catch all syscalls"
	case catchList:
		result := "catch listed syscalls:"
		for _, id := range policy.ids {
			result += " " + id.Name
		}
		return result
	default:
		panic("should never happen")
	}
}
