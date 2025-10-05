package ptrace

import (
	"fmt"
	"os/exec"
	"syscall"
)

// NOTE: ptrace is implemented as a single os-threaded server serving Tracer
// clients in arbitrary goroutines since all ptrace calls to a process (and
// its threads), including PTRACE_TRACEME in os.StartProcess / exec.Cmd.Start,
// must originate from the same os thread.
//
// https://github.com/golang/go/issues/7699
// https://github.com/golang/go/issues/43685
type Tracer struct {
	Pid int

	server *traceServer

	parent *Tracer // set for sub thread tracers
}

func StartAndAttachToProcess(cmd *exec.Cmd) (*Tracer, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Child process invokes PTRACE_TRACEME on start.
	cmd.SysProcAttr.Ptrace = true

	// Set pgid to a different group to ensure signals sent to the tracer
	// process won't be forwarded to the child command process.
	cmd.SysProcAttr.Setpgid = true

	server := newTraceServer()

	tracer := &Tracer{
		server: server,
	}

	_, err := tracer.send(request{
		opType: startOp,
		cmd:    cmd,
	})
	if err != nil {
		close(server.requestChan) // shutdown server
		return nil, err
	}

	tracer.Pid = cmd.Process.Pid
	return tracer, nil
}

func AttachToProcess(pid int) (*Tracer, error) {
	server := newTraceServer()

	tracer := &Tracer{
		Pid:    pid,
		server: server,
	}

	_, err := tracer.send(request{
		opType: attachOp,
		pid:    pid,
	})
	if err != nil {
		close(server.requestChan) // shutdown server
		return nil, err
	}

	return tracer, nil
}

func (tracer *Tracer) Close() error {
	select {
	case <-tracer.server.ctx.Done():
		return nil
	default:
		return tracer.Detach()
	}
}

func (tracer *Tracer) TraceThread(tid int) *Tracer {
	return &Tracer{
		Pid:    tid,
		server: tracer.server,
		parent: tracer,
	}
}

func (tracer *Tracer) send(req request) (response, error) {
	respChan := make(chan response, 1)
	req.pid = tracer.Pid
	req.responseChan = respChan

	select {
	case <-tracer.server.ctx.Done():
		return response{}, fmt.Errorf(
			"invalid operation. tracer has detached from process %d",
			tracer.Pid)
	case tracer.server.requestChan <- req:
		resp := <-respChan
		return resp, resp.err
	}
}

func (tracer *Tracer) Detach() error {
	if tracer.parent != nil {
		return nil
	}

	_, err := tracer.send(request{
		opType: detachOp,
	})
	return err
}

func (tracer *Tracer) Resume(signal int) error {
	_, err := tracer.send(request{
		opType: resumeOp,
		signal: signal,
	})
	return err
}

func (tracer *Tracer) SyscallTrappedResume(signal int) error {
	_, err := tracer.send(request{
		opType: syscallOp,
		signal: signal,
	})
	return err
}

func (tracer *Tracer) SetOptions(options Options) error {
	_, err := tracer.send(request{
		opType:  setOptionsOp,
		options: options,
	})
	return err
}

func (tracer *Tracer) GetGeneralRegisters() (*UserRegs, error) {
	out := &UserRegs{}
	_, err := tracer.send(request{
		opType: getRegsOp,
		regs:   out,
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (tracer *Tracer) SetGeneralRegisters(in *UserRegs) error {
	_, err := tracer.send(request{
		opType: setRegsOp,
		regs:   in,
	})
	return err
}

func (tracer *Tracer) GetFloatingPointRegisters() (*UserFPRegs, error) {
	out := &UserFPRegs{}
	_, err := tracer.send(request{
		opType: getFPRegsOp,
		fpRegs: out,
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (tracer *Tracer) SetFloatingPointRegisters(in *UserFPRegs) error {
	_, err := tracer.send(request{
		opType: setFPRegsOp,
		fpRegs: in,
	})
	return err
}

func (tracer *Tracer) PeekUserArea(offset uintptr) (uintptr, error) {
	resp, err := tracer.send(request{
		opType: peekUserOp,
		offset: offset,
	})

	return resp.registerData, err
}

func (tracer *Tracer) PokeUserArea(offset uintptr, data uintptr) error {
	_, err := tracer.send(request{
		opType:       pokeUserOp,
		offset:       offset,
		registerData: data,
	})

	return err
}

func (tracer *Tracer) PeekData(addr uintptr, data []byte) (int, error) {
	resp, err := tracer.send(request{
		opType: peekDataOp,
		addr:   addr,
		data:   data,
	})

	return resp.count, err
}

// This is equivalent to PeekData, but uses process_vm_readv instead of
// PTRACE_PEEK_DATA for reading efficiency.  This is included as part of the
// tracer since the read permission is governed by ptrace.
//
// NOTE: There's no corresponding WriteToVirtualMemory since process_vm_writev
// does not support writing to protected memory areas.
func (tracer *Tracer) ReadFromVirtualMemory(
	addr uintptr,
	data []byte,
) (
	int,
	error,
) {
	resp, err := tracer.send(request{
		opType: readMemoryOp,
		addr:   addr,
		data:   data,
	})

	return resp.count, err
}

func (tracer *Tracer) PokeData(addr uintptr, data []byte) (int, error) {
	resp, err := tracer.send(request{
		opType: pokeDataOp,
		addr:   addr,
		data:   data,
	})

	return resp.count, err
}

func (tracer *Tracer) SingleStep() error {
	_, err := tracer.send(request{
		opType: singleStepOp,
	})
	return err
}

func (tracer *Tracer) GetSigInfo() (*SigInfo, error) {
	resp, err := tracer.send(request{
		opType: getSigInfoOp,
	})
	return resp.sigInfo, err
}
