package ptrace

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
)

type opType string

const (
	start  = opType("start")
	attach = opType("attach")
	detach = opType("detach")
)

type request struct {
	opType

	cmd *exec.Cmd

	attachPid int

	signal int // resume

	options int // set options

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

	err error
}

type ptraceOp interface {
	initialize(opType, *Tracer)

	Type() opType
	process(pid int, req request) response
}

type basePtraceOp struct {
	opType
	tracer *Tracer
}

func (op basePtraceOp) Type() opType {
	return op.opType
}

func (op basePtraceOp) send(req request) (response, error) {
	req.opType = op.opType
	return op.tracer.send(req)
}

func (op *basePtraceOp) initialize(t opType, tracer *Tracer) {
	op.opType = t
	op.tracer = tracer
}

// This ensures ptrace calls to a process are goroutine-safe.
//
// NOTE: all ptrace calls to a process, including PTRACE_TRACEME in
// os.StartProcess / exec.Cmd.Start, must originate from the same os thread.
//
// https://github.com/golang/go/issues/7699
// https://github.com/golang/go/issues/43685
type Tracer struct {
	cancel func()
	ctx    context.Context

	// Reminder: requestChan is blocking.  responseChan(s) are non-blocking.
	requestChan chan request

	// NOTE: This is only used by the public interface. The processor thread
	// maintains a separate copy of the pid.
	pid int

	ops map[opType]ptraceOp

	startOp
	attachOp

	detachOp

	resumeOp
	singleStepOp

	setOptionsOp

	getRegsOp
	setRegsOp

	getFPRegsOp
	setFPRegsOp

	peekUserOp
	pokeUserOp

	peekDataOp
	pokeDataOp
	readMemoryOp
}

func (tracer *Tracer) registerOp(Type opType, op ptraceOp) {
	op.initialize(Type, tracer)
	tracer.ops[Type] = op
}

func newTracer() *Tracer {
	ctx, cancel := context.WithCancel(context.Background())

	tracer := &Tracer{
		cancel:      cancel,
		ctx:         ctx,
		requestChan: make(chan request),
		ops:         map[opType]ptraceOp{},
	}

	tracer.registerOp(start, &tracer.startOp)
	tracer.registerOp(attach, &tracer.attachOp)

	tracer.registerOp(detach, &tracer.detachOp)

	tracer.registerOp("resume", &tracer.resumeOp)
	tracer.registerOp("singleStep", &tracer.singleStepOp)

	tracer.registerOp("setOptions", &tracer.setOptionsOp)

	tracer.registerOp("getRegs", &tracer.getRegsOp)
	tracer.registerOp("setRegs", &tracer.setRegsOp)

	tracer.registerOp("getFPRegs", &tracer.getFPRegsOp)
	tracer.registerOp("setFPRegs", &tracer.setFPRegsOp)

	tracer.registerOp("peekUser", &tracer.peekUserOp)
	tracer.registerOp("pokeUser", &tracer.pokeUserOp)

	tracer.registerOp("peekData", &tracer.peekDataOp)
	tracer.registerOp("pokeData", &tracer.pokeDataOp)
	tracer.registerOp("readMemory", &tracer.readMemoryOp)

	go tracer.processRequests()
	return tracer
}

func AttachToProcess(pid int) (*Tracer, error) {
	tracer := newTracer()

	err := tracer.attach(pid)
	if err != nil {
		close(tracer.requestChan) // shutdown process thread
		return nil, err
	}

	tracer.pid = pid
	return tracer, nil
}

func StartAndAttachToProcess(cmd *exec.Cmd) (*Tracer, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Child process invokes PTRACE_TRACEME on start.
	cmd.SysProcAttr.Ptrace = true

	tracer := newTracer()

	err := tracer.start(cmd)
	if err != nil {
		close(tracer.requestChan) // shutdown process thread
		return nil, err
	}

	tracer.pid = cmd.Process.Pid
	return tracer, nil
}

func (tracer *Tracer) Pid() int {
	return tracer.pid
}

func (tracer *Tracer) processRequests() {
	runtime.LockOSThread()
	defer func() {
		tracer.cancel()
		runtime.UnlockOSThread()
	}()

	pid := tracer.Pid()
	for req := range tracer.requestChan {
		op, ok := tracer.ops[req.opType]
		if !ok {
			panic("unhandled op type: " + req.opType)
		}

		resp := op.process(pid, req)
		req.responseChan <- resp

		switch op.Type() {
		case start:
			if resp.err == nil {
				pid = req.cmd.Process.Pid
			}
		case attach:
			pid = req.attachPid
		case detach:
			return
		}
	}
}

func (tracer *Tracer) send(req request) (response, error) {
	respChan := make(chan response, 1)
	req.responseChan = respChan

	select {
	case <-tracer.ctx.Done():
		return response{}, fmt.Errorf(
			"invalid operation. tracer has detached from process %d",
			tracer.Pid())
	case tracer.requestChan <- req:
		resp := <-respChan
		return resp, resp.err
	}
}

type startOp struct {
	basePtraceOp
}

func (op startOp) process(pid int, req request) response {
	err := req.cmd.Start()
	if err != nil {
		err = fmt.Errorf("failed to start process: %w", err)
	}

	return response{
		err: err,
	}
}

func (op startOp) start(cmd *exec.Cmd) error {
	_, err := op.send(request{
		cmd: cmd,
	})
	return err
}

type attachOp struct {
	basePtraceOp
}

func (op attachOp) process(pid int, req request) response {
	err := syscall.PtraceAttach(req.attachPid)
	if err != nil {
		err = fmt.Errorf(
			"failed to attach to process %d: %w",
			req.attachPid,
			err)
	}

	return response{
		err: err,
	}
}

func (op attachOp) attach(pid int) error {
	_, err := op.send(request{
		attachPid: pid,
	})
	return err
}

type detachOp struct {
	basePtraceOp
}

func (op detachOp) process(pid int, req request) response {
	err := syscall.PtraceDetach(pid)
	if err != nil {
		err = fmt.Errorf("failed to detach from process %d: %w", pid, err)
	}

	return response{
		err: err,
	}
}

func (op detachOp) Detach() error {
	_, err := op.send(request{})
	return err
}

type resumeOp struct {
	basePtraceOp
}

func (op resumeOp) process(pid int, req request) response {
	err := syscall.PtraceCont(pid, req.signal)
	if err != nil {
		err = fmt.Errorf("failed to resume process %d: %w", pid, err)
	}

	return response{
		err: err,
	}
}

func (op resumeOp) Resume(signal int) error {
	_, err := op.send(request{
		signal: signal,
	})
	return err
}

type setOptionsOp struct {
	basePtraceOp
}

func (op setOptionsOp) process(pid int, req request) response {
	err := syscall.PtraceSetOptions(pid, req.options)
	if err != nil {
		err = fmt.Errorf("failed to set options for process %d: %w", pid, err)
	}

	return response{
		err: err,
	}
}

func (op setOptionsOp) SetOptions(options int) error {
	_, err := op.send(request{
		options: options,
	})
	return err
}

type getRegsOp struct {
	basePtraceOp
}

func (op getRegsOp) process(pid int, req request) response {
	err := syscall.PtraceGetRegs(pid, req.regs)
	if err != nil {
		err = fmt.Errorf(
			"failed to get general register values from process %d: %w",
			pid,
			err)
	}

	return response{
		err: err,
	}
}

func (op getRegsOp) GetGeneralRegisters() (*UserRegs, error) {
	out := &UserRegs{}
	_, err := op.send(request{
		regs: out,
	})
	return out, err
}

type setRegsOp struct {
	basePtraceOp
}

func (op setRegsOp) process(pid int, req request) response {
	err := syscall.PtraceSetRegs(pid, req.regs)
	if err != nil {
		err = fmt.Errorf(
			"failed to set general register values for process %d: %w",
			pid,
			err)
	}

	return response{
		err: err,
	}
}

func (op setRegsOp) SetGeneralRegisters(in *UserRegs) error {
	_, err := op.send(request{
		regs: in,
	})
	return err
}

type getFPRegsOp struct {
	basePtraceOp
}

func (op getFPRegsOp) process(pid int, req request) response {
	err := getFPRegs(pid, req.fpRegs)
	if err != nil {
		err = fmt.Errorf(
			"failed to get floating point register values from process %d: %w",
			pid,
			err)
	}

	return response{
		err: err,
	}
}

func (op getFPRegsOp) GetFloatingPointRegisters() (*UserFPRegs, error) {
	out := &UserFPRegs{}
	_, err := op.send(request{
		fpRegs: out,
	})
	return out, err
}

type setFPRegsOp struct {
	basePtraceOp
}

func (op setFPRegsOp) process(pid int, req request) response {
	err := setFPRegs(pid, req.fpRegs)
	if err != nil {
		err = fmt.Errorf(
			"failed to set floating point register values from process %d: %w",
			pid,
			err)
	}

	return response{
		err: err,
	}
}

func (op setFPRegsOp) SetFloatingPointRegisters(in *UserFPRegs) error {
	_, err := op.send(request{
		fpRegs: in,
	})
	return err
}

type peekUserOp struct {
	basePtraceOp
}

func (op peekUserOp) process(pid int, req request) response {
	data, err := peekUserArea(pid, req.offset)

	resp := response{}
	if err == nil {
		resp.registerData = data
	} else {
		resp.err = fmt.Errorf(
			"failed to peek user area (%d) for process %d: %w",
			req.offset,
			pid,
			err)
	}

	return resp
}

func (op peekUserOp) PeekUserArea(offset uintptr) (uintptr, error) {
	resp, err := op.send(request{
		offset: offset,
	})

	return resp.registerData, err
}

type pokeUserOp struct {
	basePtraceOp
}

func (op pokeUserOp) process(pid int, req request) response {
	err := pokeUserArea(pid, req.offset, req.registerData)
	if err != nil {
		err = fmt.Errorf(
			"failed to poke user area (%d ; %d) for process %d: %w",
			req.offset,
			req.registerData,
			pid,
			err)
	}

	return response{
		err: err,
	}
}

func (op pokeUserOp) PokeUserArea(offset uintptr, data uintptr) error {
	_, err := op.send(request{
		offset:       offset,
		registerData: data,
	})

	return err
}

type peekDataOp struct {
	basePtraceOp
}

func (op peekDataOp) process(pid int, req request) response {
	count, err := syscall.PtracePeekData(pid, req.addr, req.data)
	if err != nil {
		err = fmt.Errorf(
			"failed to peek data (%d ; %d) for process %d: %w",
			req.addr,
			len(req.data),
			pid,
			err)
	}

	return response{
		count: count,
		err:   err,
	}
}

func (op peekDataOp) PeekData(addr uintptr, data []byte) (int, error) {
	resp, err := op.send(request{
		addr: addr,
		data: data,
	})

	return resp.count, err
}

// This is equivalent to PeekData, but uses process_vm_readv instead of
// PTRACE_PEEK_DATA for reading efficiency.  This is included as part of the
// tracer since the read permission is governed by ptrace.
//
// NOTE: There's no corresponding WriteToVirtualMemory since process_vm_writev
// does not support writing to protected memory areas.
type readMemoryOp struct {
	basePtraceOp
}

func (op readMemoryOp) process(pid int, req request) response {
	count, err := readVirtualMemory(pid, req.addr, req.data)
	if err != nil {
		err = fmt.Errorf(
			"failed to process_vm_readv at %d (%d) from process %d: %w",
			req.addr,
			len(req.data),
			pid,
			err)
	}

	return response{
		count: count,
		err:   err,
	}
}

func (op readMemoryOp) ReadFromVirtualMemory(
	addr uintptr,
	data []byte,
) (
	int,
	error,
) {
	resp, err := op.send(request{
		addr: addr,
		data: data,
	})

	return resp.count, err
}

type pokeDataOp struct {
	basePtraceOp
}

func (op pokeDataOp) process(pid int, req request) response {
	count, err := syscall.PtracePokeData(pid, req.addr, req.data)
	if err != nil {
		err = fmt.Errorf(
			"failed to poke data (%d ; %d) for process %d: %w",
			req.addr,
			len(req.data),
			pid,
			err)
	}

	return response{
		count: count,
		err:   err,
	}
}

func (op pokeDataOp) PokeData(addr uintptr, data []byte) (int, error) {
	resp, err := op.send(request{
		addr: addr,
		data: data,
	})

	return resp.count, err
}

type singleStepOp struct {
	basePtraceOp
}

func (op singleStepOp) process(pid int, req request) response {
	err := syscall.PtraceSingleStep(pid)
	if err != nil {
		err = fmt.Errorf("failed to single step process %d: %w", pid, err)
	}

	return response{
		err: err,
	}
}

func (op singleStepOp) SingleStep() error {
	_, err := op.send(request{})
	return err
}
