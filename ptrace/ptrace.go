package ptrace

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
)

type requestType string

const (
	start      = requestType("start")
	attach     = requestType("attach")
	detach     = requestType("detach")
	resume     = requestType("resume")
	setoptions = requestType("setoptions")
	getregs    = requestType("getregs")
	setregs    = requestType("setregs")
	getfpregs  = requestType("getfpregs")
	setfpregs  = requestType("setfpregs")
	peekuser   = requestType("peekuser")
	pokeuser   = requestType("pokeuser")
)

type request struct {
	requestType

	cmd *exec.Cmd

	signal int // resume

	options int // set options

	regs *UserRegs // get/set regs

	fpRegs *UserFPRegs // get/set fp regs

	offset uintptr // peek/poke user area
	data   uintptr // poke user area

	responseChan chan response
}

type response struct {
	data uintptr
	err  error
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

	mutex sync.Mutex

	_pid int // guarded by mutex
}

func newTracer(pid int) *Tracer {
	ctx, cancel := context.WithCancel(context.Background())

	tracer := &Tracer{
		cancel:      cancel,
		ctx:         ctx,
		requestChan: make(chan request),
		_pid:        pid,
	}

	go tracer.processRequests()
	return tracer
}

func AttachToProcess(pid int) (*Tracer, error) {
	tracer := newTracer(pid)

	_, err := tracer.send(request{
		requestType: attach,
	})
	if err != nil {
		close(tracer.requestChan) // shutdown process thread
		return nil, err
	}

	return tracer, nil
}

func StartAndAttachToProcess(cmd *exec.Cmd) (*Tracer, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Child process invokes PTRACE_TRACEME on start.
	cmd.SysProcAttr.Ptrace = true

	tracer := newTracer(0)

	_, err := tracer.send(request{
		requestType: start,
		cmd:         cmd,
	})
	if err != nil {
		close(tracer.requestChan) // shutdown process thread
		return nil, err
	}

	return tracer, nil
}

func (tracer *Tracer) Pid() int {
	tracer.mutex.Lock()
	defer tracer.mutex.Unlock()

	return tracer._pid
}

func (tracer *Tracer) setPid(pid int) {
	tracer.mutex.Lock()
	defer tracer.mutex.Unlock()

	tracer._pid = pid
}

func (tracer *Tracer) processRequests() {
	runtime.LockOSThread()
	defer func() {
		tracer.cancel()
		runtime.UnlockOSThread()
	}()

	pid := tracer.Pid()
	for req := range tracer.requestChan {
		switch req.requestType {
		case start:
			err := req.cmd.Start()
			if err != nil {
				err = fmt.Errorf("failed to start process: %w", err)
			} else {
				pid = req.cmd.Process.Pid
				tracer.setPid(pid)
			}

			req.responseChan <- response{
				err: err,
			}
		case attach:
			err := syscall.PtraceAttach(tracer.Pid())
			if err != nil {
				err = fmt.Errorf("failed to attach to process %d: %w", pid, err)
			}

			req.responseChan <- response{
				err: err,
			}
		case detach:
			err := syscall.PtraceDetach(pid)
			if err != nil {
				err = fmt.Errorf("failed to detach from process %d: %w", pid, err)
			}

			req.responseChan <- response{
				err: err,
			}

			return
		case resume:
			err := syscall.PtraceCont(pid, req.signal)
			if err != nil {
				err = fmt.Errorf("failed to resume process %d: %w", pid, err)
			}

			req.responseChan <- response{
				err: err,
			}
		case setoptions:
			err := syscall.PtraceSetOptions(pid, req.options)
			if err != nil {
				err = fmt.Errorf("failed to set options for process %d: %w", pid, err)
			}

			req.responseChan <- response{
				err: err,
			}
		case getregs:
			err := syscall.PtraceGetRegs(pid, req.regs)
			if err != nil {
				err = fmt.Errorf(
					"failed to get general register values from process %d: %w",
					pid,
					err)
			}

			req.responseChan <- response{
				err: err,
			}
		case setregs:
			err := syscall.PtraceSetRegs(pid, req.regs)
			if err != nil {
				err = fmt.Errorf(
					"failed to set general register values for process %d: %w",
					pid,
					err)
			}

			req.responseChan <- response{
				err: err,
			}
		case getfpregs:
			err := getFPRegs(pid, req.fpRegs)
			if err != nil {
				err = fmt.Errorf(
					"failed to get floating point register values from process %d: %w",
					pid,
					err)
			}

			req.responseChan <- response{
				err: err,
			}
		case setfpregs:
			err := setFPRegs(pid, req.fpRegs)
			if err != nil {
				err = fmt.Errorf(
					"failed to set floating point register values from process %d: %w",
					pid,
					err)
			}

			req.responseChan <- response{
				err: err,
			}
		case peekuser:
			data, err := peekUserArea(pid, req.offset)

			resp := response{}
			if err == nil {
				resp.data = data
			} else {
				resp.err = fmt.Errorf(
					"failed to peek user area (%d) for process %d: %w",
					req.offset,
					pid,
					err)
			}

			req.responseChan <- resp
		case pokeuser:
			err := pokeUserArea(pid, req.offset, req.data)
			if err != nil {
				err = fmt.Errorf(
					"failed to poke user area (%d ; %d) for process %d: %w",
					req.offset,
					req.data,
					pid,
					err)
			}

			req.responseChan <- response{
				err: err,
			}
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

func (tracer *Tracer) Detach() error {
	_, err := tracer.send(request{
		requestType: detach,
	})
	return err
}

func (tracer *Tracer) Resume(signal int) error {
	_, err := tracer.send(request{
		requestType: resume,
		signal:      signal,
	})
	return err
}

func (tracer *Tracer) SetOptions(options int) error {
	_, err := tracer.send(request{
		requestType: setoptions,
		options:     options,
	})
	return err
}

func (tracer *Tracer) GetGeneralRegisters() (*UserRegs, error) {
	out := &UserRegs{}
	_, err := tracer.send(request{
		requestType: getregs,
		regs:        out,
	})
	return out, err
}

func (tracer *Tracer) SetGeneralRegisters(in *UserRegs) error {
	_, err := tracer.send(request{
		requestType: setregs,
		regs:        in,
	})
	return err
}

func (tracer *Tracer) GetFloatingPointRegisters() (*UserFPRegs, error) {
	out := &UserFPRegs{}
	_, err := tracer.send(request{
		requestType: getfpregs,
		fpRegs:      out,
	})
	return out, err
}

func (tracer *Tracer) SetFloatingPointRegisters(in *UserFPRegs) error {
	_, err := tracer.send(request{
		requestType: setfpregs,
		fpRegs:      in,
	})
	return err
}

func (tracer *Tracer) PeekUserArea(offset uintptr) (uintptr, error) {
	resp, err := tracer.send(request{
		requestType: peekuser,
		offset:      offset,
	})

	return resp.data, err
}

func (tracer *Tracer) PokeUserArea(offset uintptr, data uintptr) error {
	_, err := tracer.send(request{
		requestType: pokeuser,
		offset:      offset,
		data:        data,
	})

	return err
}
