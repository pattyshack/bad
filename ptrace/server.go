package ptrace

import (
	"context"
	"fmt"
	"runtime"
	"syscall"
)

type traceServer struct {
	cancel func()
	ctx    context.Context

	// Reminder: requestChan is blocking. responseChan(s) are non-blocking.
	requestChan chan request
}

func newTraceServer() *traceServer {
	ctx, cancel := context.WithCancel(context.Background())

	server := &traceServer{
		cancel:      cancel,
		ctx:         ctx,
		requestChan: make(chan request),
	}

	go server.processRequests()
	return server
}

func (server *traceServer) processRequests() {
	runtime.LockOSThread()
	defer func() {
		server.cancel()
		runtime.UnlockOSThread()
	}()

	for req := range server.requestChan {
		switch req.opType {
		case startOp:
			req.responseChan <- server.start(req)
		case attachOp:
			req.responseChan <- server.attach(req)
		case detachOp:
			req.responseChan <- server.detach(req)
			return
		case resumeOp:
			req.responseChan <- server.resume(req)
		case syscallOp:
			req.responseChan <- server.syscallTrappedResume(req)
		case singleStepOp:
			req.responseChan <- server.singleStep(req)
		case setOptionsOp:
			req.responseChan <- server.setOptions(req)
		case getRegsOp:
			req.responseChan <- server.getRegs(req)
		case setRegsOp:
			req.responseChan <- server.setRegs(req)
		case getFPRegsOp:
			req.responseChan <- server.getFPRegs(req)
		case setFPRegsOp:
			req.responseChan <- server.setFPRegs(req)
		case peekUserOp:
			req.responseChan <- server.peekUser(req)
		case pokeUserOp:
			req.responseChan <- server.pokeUser(req)
		case peekDataOp:
			req.responseChan <- server.peekData(req)
		case pokeDataOp:
			req.responseChan <- server.pokeData(req)
		case readMemoryOp:
			req.responseChan <- server.readMemory(req)
		case getSigInfoOp:
			req.responseChan <- server.getSigInfo(req)
		}
	}
}

func (server *traceServer) start(req request) response {
	err := req.cmd.Start()
	if err != nil {
		err = fmt.Errorf("failed to start process: %w", err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) attach(req request) response {
	err := syscall.PtraceAttach(req.pid)
	if err != nil {
		err = fmt.Errorf("failed to attach to process %d: %w", req.pid, err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) detach(req request) response {
	err := syscall.PtraceDetach(req.pid)
	if err != nil {
		err = fmt.Errorf("failed to detach from process %d: %w", req.pid, err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) resume(req request) response {
	err := syscall.PtraceCont(req.pid, req.signal)
	if err != nil {
		err = fmt.Errorf("failed to resume process %d: %w", req.pid, err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) syscallTrappedResume(req request) response {
	err := syscall.PtraceSyscall(req.pid, req.signal)
	if err != nil {
		err = fmt.Errorf(
			"failed to (syscall-trapped) resume process %d: %w",
			req.pid,
			err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) setOptions(req request) response {
	err := syscall.PtraceSetOptions(req.pid, int(req.options))
	if err != nil {
		err = fmt.Errorf("failed to set options for process %d: %w", req.pid, err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) getRegs(req request) response {
	err := syscall.PtraceGetRegs(req.pid, req.regs)
	if err != nil {
		err = fmt.Errorf(
			"failed to get general register values from process %d: %w",
			req.pid,
			err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) setRegs(req request) response {
	err := syscall.PtraceSetRegs(req.pid, req.regs)
	if err != nil {
		err = fmt.Errorf(
			"failed to set general register values for process %d: %w",
			req.pid,
			err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) getFPRegs(req request) response {
	err := getFPRegs(req.pid, req.fpRegs)
	if err != nil {
		err = fmt.Errorf(
			"failed to get floating point register values from process %d: %w",
			req.pid,
			err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) setFPRegs(req request) response {
	err := setFPRegs(req.pid, req.fpRegs)
	if err != nil {
		err = fmt.Errorf(
			"failed to set floating point register values from process %d: %w",
			req.pid,
			err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) peekUser(req request) response {
	data, err := peekUserArea(req.pid, req.offset)

	resp := response{}
	if err == nil {
		resp.registerData = data
	} else {
		resp.err = fmt.Errorf(
			"failed to peek user area (%d) for process %d: %w",
			req.offset,
			req.pid,
			err)
	}

	return resp
}

func (server *traceServer) pokeUser(req request) response {
	err := pokeUserArea(req.pid, req.offset, req.registerData)
	if err != nil {
		err = fmt.Errorf(
			"failed to poke user area (%d ; %d) for process %d: %w",
			req.offset,
			req.registerData,
			req.pid,
			err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) peekData(req request) response {
	count, err := syscall.PtracePeekData(req.pid, req.addr, req.data)
	if err != nil {
		err = fmt.Errorf(
			"failed to peek data (%d ; %d) for process %d: %w",
			req.addr,
			len(req.data),
			req.pid,
			err)
	}

	return response{
		count: count,
		err:   err,
	}
}

// This is equivalent to PeekData, but uses process_vm_readv instead of
// PTRACE_PEEK_DATA for reading efficiency.  This is included as part of the
// tracer since the read permission is governed by ptrace.
//
// NOTE: There's no corresponding WriteToVirtualMemory since process_vm_writev
// does not support writing to protected memory areas.
func (server *traceServer) readMemory(req request) response {
	count, err := readVirtualMemory(req.pid, req.addr, req.data)
	if err != nil {
		err = fmt.Errorf(
			"failed to process_vm_readv at %d (%d) from process %d: %w",
			req.addr,
			len(req.data),
			req.pid,
			err)
	}

	return response{
		count: count,
		err:   err,
	}
}

func (server *traceServer) pokeData(req request) response {
	count, err := syscall.PtracePokeData(req.pid, req.addr, req.data)
	if err != nil {
		err = fmt.Errorf(
			"failed to poke data (%d ; %d) for process %d: %w",
			req.addr,
			len(req.data),
			req.pid,
			err)
	}

	return response{
		count: count,
		err:   err,
	}
}

func (server *traceServer) singleStep(req request) response {
	err := syscall.PtraceSingleStep(req.pid)
	if err != nil {
		err = fmt.Errorf("failed to single step process %d: %w", req.pid, err)
	}

	return response{
		err: err,
	}
}

func (server *traceServer) getSigInfo(req request) response {
	out := &SigInfo{}
	err := getSigInfo(req.pid, out)
	if err != nil {
		out = nil
		err = fmt.Errorf(
			"failed to get signal information from process %d: %w",
			req.pid,
			err)
	}

	return response{
		sigInfo: out,
		err:     err,
	}
}
