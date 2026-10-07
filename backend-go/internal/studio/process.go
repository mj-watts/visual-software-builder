package studio

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type emitFunc func(string, string) error

// Run argv directly, with isolated environment and a process group for descendants.
func runProcess(ctx context.Context, argv []string, cwd, input string, env []string, emit emitFunc) error {
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(child, argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = env
	cmd.Stdin = strings.NewReader(input)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	defer syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	done := make(chan struct{})
	defer close(done)
	go killAfterCancel(child, cmd.Process.Pid, done)
	var budget atomic.Int64
	var mu sync.Mutex
	safeEmit := func(k, v string) error { mu.Lock(); defer mu.Unlock(); return emit(k, v) }
	results := make(chan error, 2)
	for _, stream := range []struct {
		reader io.Reader
		kind   string
	}{{stdout, "stdout"}, {stderr, "stderr"}} {
		go func() {
			err := pump(stream.reader, stream.kind, &budget, safeEmit)
			if err != nil {
				cancel()
			}
			results <- err
		}()
	}
	first, second := <-results, <-results
	waitErr := cmd.Wait()
	return processResult(ctx, first, second, waitErr)
}
func processResult(ctx context.Context, first, second, waitErr error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if first != nil {
		return first
	}
	if second != nil {
		return second
	}
	if waitErr != nil {
		return fmt.Errorf("Agent process exited unsuccessfully. See run activity for details: %w", waitErr)
	}
	return nil
}

func killAfterCancel(ctx context.Context, pid int, done <-chan struct{}) {
	select {
	case <-done:
		return
	case <-ctx.Done():
	}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
		return
	case <-timer.C:
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}
func pump(reader io.Reader, kind string, budget *atomic.Int64, emit emitFunc) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1000000)
	for scanner.Scan() {
		line := scanner.Text()
		if budget.Add(int64(len(line)+1)) > 5000000 {
			return fmt.Errorf("Agent output exceeded the 5 MB limit.")
		}
		if err := emit(kind, line); err != nil {
			return err
		}
	}
	return scanner.Err()
}
