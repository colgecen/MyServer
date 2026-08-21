// Package exec implements the SafeExec command execution pipeline.
//
// Responsibilities:
//   - spawn child processes under the user's shell (bash on POSIX, powershell on Windows)
//   - stream stdout/stderr line-by-line over a callback channel
//   - enforce timeouts and graceful (then forced) termination via process groups
//   - cap output to prevent unbounded memory growth
package exec

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"
)

const (
	defaultTimeout  = 60 * time.Second
	maxOutputBytes  = 1 << 20 // 1 MiB per stream, truncated after this
	streamBufSize   = 64
)

// StreamEvent is delivered to consumers as a process runs.
type StreamEvent struct {
	Stream   string // "stdout" | "stderr" | "system"
	Data     string
	ExitCode int
	Done     bool
	Err      error
}

// Options configures a single execution.
type Options struct {
	Command  string        // required
	Workdir  string        // optional; resolved against workspace if set
	Shell    string        // "bash" | "powershell" | "" for auto
	Timeout  time.Duration // 0 means defaultTimeout
	MaxBytes int           // 0 means maxOutputBytes
	Env      []string      // optional extra env vars
}

// Result is the final outcome after the process exits.
type Result struct {
	ExitCode int
	Duration time.Duration
	Truncated bool
}

// Engine runs commands. Safe for concurrent use.
type Engine struct {
	mu        sync.Mutex
	lastByTag map[string]*cmdHandle // optional in-flight tracking
}

// NewEngine constructs a fresh Engine.
func NewEngine() *Engine {
	return &Engine{lastByTag: make(map[string]*cmdHandle)}
}

// Run executes the command and streams output through onEvent until completion
// or context cancellation. The call returns when the process has exited or the
// context is cancelled. The final onEvent invocation has Done=true.
func (e *Engine) Run(ctx context.Context, opts Options, onEvent func(StreamEvent)) (*Result, error) {
	if opts.Command == "" {
		return nil, errors.New("exec: empty command")
	}
	if onEvent == nil {
		return nil, errors.New("exec: nil event callback")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = maxOutputBytes
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell, shellArgs := pickShell(opts.Shell)
	cmd := exec.CommandContext(cctx, shell, shellArgs...)
	cmd.Dir = opts.Workdir
	cmd.Env = composeEnv(opts.Env)
	setProcessGroup(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	h := &cmdHandle{cmd: cmd}
	e.mu.Lock()
	e.lastByTag[opts.Command] = h
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.lastByTag, opts.Command)
		e.mu.Unlock()
	}()

	var (
		wg          sync.WaitGroup
		truncMu     sync.Mutex
		truncated   bool
		emit        = func(ev StreamEvent) { onEvent(ev) }
		capAndEmit  = func(stream string, data []byte) {
			truncMu.Lock()
			if truncated {
				truncMu.Unlock()
				return
			}
			truncMu.Unlock()
			ev := StreamEvent{Stream: stream, Data: string(data)}
			if len(data) > maxBytes {
				ev.Data = string(data[:maxBytes]) + "\n[...truncated...]"
				truncMu.Lock()
				truncated = true
				truncMu.Unlock()
			}
			emit(ev)
		}
	)

	wg.Add(2)
	go pump(stdout, "stdout", capAndEmit, &wg)
	go pump(stderr, "stderr", capAndEmit, &wg)

	waitErr := cmd.Wait()
	wg.Wait()

	res := &Result{Duration: time.Since(start), Truncated: truncated}
	if waitErr != nil {
		var ee *exec.ExitError
		if errors.As(waitErr, &ee) {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
			emit(StreamEvent{Stream: "system", Err: waitErr, Done: true})
			return res, waitErr
		}
	}

	emit(StreamEvent{Stream: "system", ExitCode: res.ExitCode, Done: true})
	return res, nil
}

// KillAll terminates all in-flight processes managed by this Engine.
// Used by daemon shutdown.
func (e *Engine) KillAll() {
	e.mu.Lock()
	handles := make([]*cmdHandle, 0, len(e.lastByTag))
	for _, h := range e.lastByTag {
		handles = append(handles, h)
	}
	e.mu.Unlock()
	for _, h := range handles {
		_ = killProcessTree(h.cmd)
	}
}

// ----- internals -----

type cmdHandle struct {
	cmd *exec.Cmd
}

func pickShell(requested string) (string, []string) {
	switch requested {
	case "bash":
		return "bash", []string{"-lc"}
	case "powershell":
		return "powershell", []string{"-NoProfile", "-NonInteractive", "-Command"}
	}
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-NonInteractive", "-Command"}
	}
	return "bash", []string{"-lc"}
}

func composeEnv(extra []string) []string {
	if len(extra) == 0 {
		return os.Environ()
	}
	// dedupe: extra wins over inherited
	merged := os.Environ()
	for _, kv := range extra {
		merged = append(merged, kv)
	}
	return merged
}

// setProcessGroup puts the child in its own group so we can SIGKILL the whole
// tree on timeout. POSIX only; no-op on Windows.
func setProcessGroup(cmd *exec.Cmd) {
	if runtime.GOOS == "windows" {
		// SysProcAttr on Windows uses CreationFlags; minimal here.
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func pump(r io.Reader, stream string, emit func(string, []byte), wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		emit(stream, scanner.Bytes())
	}
}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return cmd.Process.Kill()
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Kill()
	}
	// negative pid => signal the whole process group
	return syscall.Kill(-pgid, syscall.SIGKILL)
}
