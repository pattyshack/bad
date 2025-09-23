package debugger

import (
	"context"
	"fmt"
	"os"
	osSignal "os/signal"
	"syscall"
)

type Signaler struct {
	pid int

	ctx    context.Context
	cancel func()
}

func NewSignaler(pid int) *Signaler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Signaler{
		pid:    pid,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (signaler *Signaler) Close() error {
	signaler.cancel()
	return nil
}

func (signaler *Signaler) ForwardToProcess(signal syscall.Signal) {
	signalChan := make(chan os.Signal)
	osSignal.Notify(signalChan, signal)

	go func() {
		for {
			select {
			case <-signaler.ctx.Done():
				return
			case <-signalChan:
				err := signaler.ToProcess(signal)
				if err != nil {
					panic(err)
				}
			}
		}
	}()
}

func (signaler *Signaler) ForwardInterruptToProcess() {
	signaler.ForwardToProcess(syscall.SIGINT)
}

func (signaler *Signaler) ToProcess(signal syscall.Signal) error {
	err := syscall.Kill(signaler.pid, signal)
	if err != nil {
		return fmt.Errorf("failed to signal to process %d (%v): %w",
			signaler.pid,
			signal,
			err)
	}

	return nil
}

func (signaler *Signaler) ContinueToProcess() error {
	return signaler.ToProcess(syscall.SIGCONT)
}

func (signaler *Signaler) StopToProcess() error {
	return signaler.ToProcess(syscall.SIGSTOP)
}

func (signaler *Signaler) KillToProcess() error {
	return signaler.ToProcess(syscall.SIGKILL)
}

func (signaler *Signaler) FromProcess() (syscall.WaitStatus, error) {
	// NOTE: golang does not support waitpid
	var waitStatus syscall.WaitStatus
	_, err := syscall.Wait4(signaler.pid, &waitStatus, 0, nil)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to wait for process %d: %w",
			signaler.pid,
			err)
	}

	return waitStatus, nil
}
