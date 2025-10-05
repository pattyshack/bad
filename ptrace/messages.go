package ptrace

import (
	"os/exec"
)

type opType string

const (
	startOp      = opType("start")
	attachOp     = opType("attach")
	detachOp     = opType("detach")
	resumeOp     = opType("resume")
	syscallOp    = opType("syscall")
	singleStepOp = opType("singleStep")
	setOptionsOp = opType("setOptions")
	getRegsOp    = opType("getRegs")
	setRegsOp    = opType("setRegs")
	getFPRegsOp  = opType("getFPRegs")
	setFPRegsOp  = opType("setFPRegs")
	peekUserOp   = opType("peekUser")
	pokeUserOp   = opType("pokeUser")
	peekDataOp   = opType("peekData")
	pokeDataOp   = opType("pokeData")
	readMemoryOp = opType("readMemory")
	getSigInfoOp = opType("getSigInfo")
)

type request struct {
	opType

	cmd *exec.Cmd // only used by start

	pid int // used by all except start

	signal int // resume

	options Options // set options

	regs *UserRegs // get/set regs

	fpRegs *UserFPRegs // get/set fp regs

	offset       uintptr // peek/poke user area
	registerData uintptr // poke user area

	addr uintptr // peek/poke data
	data []byte  // peek/poke data

	responseChan chan response
}

type response struct {
	registerData uintptr // peek user area

	count int // peek/poke data

	sigInfo *SigInfo // get sig info

	err error
}
