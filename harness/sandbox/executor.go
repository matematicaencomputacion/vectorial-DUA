package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Policy defines isolation constraints for student cell execution.
type Policy struct {
	AllowedRuntimes []string
	Timeout         time.Duration
	MaxOutputBytes  int
	WorkDirRoot     string
}

// DefaultPolicy returns a conservative local sandbox policy.
func DefaultPolicy() Policy {
	return Policy{
		AllowedRuntimes: []string{"python", "python3", "node"},
		Timeout:         3 * time.Second,
		MaxOutputBytes:  64 * 1024,
		WorkDirRoot:     os.TempDir(),
	}
}

// Request is a single sandboxed execution request.
type Request struct {
	Runtime string
	Source  string
	Args    []string
}

// Violation codes for policy breaches.
const (
	ViolationNone          = ""
	ViolationTimeout       = "timeout"
	ViolationRuntimeDenied = "runtime_denied"
	ViolationOutputLimit   = "output_limit"
	ViolationExecFailed    = "exec_failed"
)

// Result captures sandboxed execution outcome.
type Result struct {
	Runtime    string        `json:"runtime"`
	ExitCode   int           `json:"exit_code"`
	Duration   time.Duration `json:"duration"`
	Stdout     string        `json:"stdout"`
	Stderr     string        `json:"stderr"`
	Violation  string        `json:"violation,omitempty"`
	WorkDir    string        `json:"work_dir,omitempty"`
}

// Executor runs student code under policy.
type Executor struct {
	Policy Policy
}

// Run executes source in an isolated temp directory with timeout.
func (e *Executor) Run(ctx context.Context, req Request) (Result, error) {
	pol := e.Policy
	if pol.Timeout == 0 {
		pol = DefaultPolicy()
	}

	runtime := strings.TrimSpace(req.Runtime)
	if !allowed(pol.AllowedRuntimes, runtime) {
		return Result{Runtime: runtime, Violation: ViolationRuntimeDenied, ExitCode: -1}, nil
	}

	dir, err := os.MkdirTemp(pol.WorkDirRoot, "avlp-sandbox-*")
	if err != nil {
		return Result{}, fmt.Errorf("workdir: %w", err)
	}
	defer os.RemoveAll(dir)

	filename, err := writeSource(dir, runtime, req.Source)
	if err != nil {
		return Result{}, err
	}

	runCtx, cancel := context.WithTimeout(ctx, pol.Timeout)
	defer cancel()

	cmd, err := buildCmd(runCtx, runtime, filename, req.Args)
	if err != nil {
		return Result{Runtime: runtime, Violation: ViolationExecFailed, ExitCode: -1, WorkDir: dir}, err
	}
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{buf: &stdout, limit: pol.MaxOutputBytes}
	cmd.Stderr = &limitedWriter{buf: &stderr, limit: pol.MaxOutputBytes}

	start := time.Now()
	err = cmd.Run()
	dur := time.Since(start)

	res := Result{
		Runtime:  runtime,
		Duration: dur,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		WorkDir:  dir,
	}

	if lw, ok := cmd.Stdout.(*limitedWriter); ok && lw.exceeded {
		res.Violation = ViolationOutputLimit
	}
	if lw, ok := cmd.Stderr.(*limitedWriter); ok && lw.exceeded && res.Violation == "" {
		res.Violation = ViolationOutputLimit
	}

	if runCtx.Err() == context.DeadlineExceeded {
		res.Violation = ViolationTimeout
		res.ExitCode = -1
		return res, nil
	}

	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.ExitCode = ee.ExitCode()
			return res, nil
		}
		res.Violation = ViolationExecFailed
		res.ExitCode = -1
		return res, nil
	}
	res.ExitCode = 0
	return res, nil
}

func allowed(list []string, runtime string) bool {
	for _, a := range list {
		if strings.EqualFold(a, runtime) {
			return true
		}
	}
	return false
}

func writeSource(dir, runtime, source string) (string, error) {
	name := "cell.py"
	switch {
	case strings.HasPrefix(strings.ToLower(runtime), "node"):
		name = "cell.js"
	case strings.HasPrefix(strings.ToLower(runtime), "python"):
		name = "cell.py"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func buildCmd(ctx context.Context, runtime, filename string, args []string) (*exec.Cmd, error) {
	all := append([]string{filename}, args...)
	switch {
	case strings.HasPrefix(strings.ToLower(runtime), "python"):
		bin := runtime
		if runtime == "python" {
			if _, err := exec.LookPath("python"); err != nil {
				bin = "python3"
			}
		}
		return exec.CommandContext(ctx, bin, all...), nil
	case strings.HasPrefix(strings.ToLower(runtime), "node"):
		return exec.CommandContext(ctx, "node", all...), nil
	default:
		return nil, fmt.Errorf("unsupported runtime %q", runtime)
	}
}

type limitedWriter struct {
	buf      *bytes.Buffer
	limit    int
	exceeded bool
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	remaining := w.limit - w.buf.Len()
	if remaining <= 0 {
		w.exceeded = true
		return len(p), nil
	}
	if len(p) > remaining {
		w.exceeded = true
		_, _ = w.buf.Write(p[:remaining])
		return len(p), nil
	}
	return w.buf.Write(p)
}
