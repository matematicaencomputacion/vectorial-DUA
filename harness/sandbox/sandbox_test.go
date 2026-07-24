package sandbox_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/harness/sandbox"
)

func TestSandboxRuntimeDenied(t *testing.T) {
	ex := sandbox.Executor{Policy: sandbox.DefaultPolicy()}
	res, err := ex.Run(context.Background(), sandbox.Request{
		Runtime: "bash",
		Source:  "echo hi",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Violation != sandbox.ViolationRuntimeDenied {
		t.Fatalf("expected runtime_denied, got %+v", res)
	}
}

func TestSandboxHappyPathOrSkip(t *testing.T) {
	ex := sandbox.Executor{Policy: sandbox.DefaultPolicy()}
	if _, err := exec.LookPath("node"); err == nil {
		res, err := ex.Run(context.Background(), sandbox.Request{
			Runtime: "node",
			Source:  "console.log('ok')",
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.ExitCode != 0 || res.Violation != "" {
			t.Fatalf("unexpected result: %+v", res)
		}
		return
	}
	if _, err := exec.LookPath("python"); err == nil || lookPython3() {
		runtime := "python"
		if _, err := exec.LookPath("python"); err != nil {
			runtime = "python3"
		}
		res, err := ex.Run(context.Background(), sandbox.Request{
			Runtime: runtime,
			Source:  "print('ok')",
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.ExitCode != 0 || res.Violation != "" {
			t.Fatalf("unexpected result: %+v", res)
		}
		return
	}
	t.Skip("no node/python runtime available")
}

func TestSandboxTimeout(t *testing.T) {
	runtime := ""
	source := ""
	if _, err := exec.LookPath("python"); err == nil {
		runtime = "python"
		source = "import time\ntime.sleep(30)"
	} else if lookPython3() {
		runtime = "python3"
		source = "import time\ntime.sleep(30)"
	} else {
		t.Skip("python required for timeout test (avoids orphan node busy-loops on Windows)")
	}

	pol := sandbox.DefaultPolicy()
	pol.Timeout = 250 * time.Millisecond
	ex := sandbox.Executor{Policy: pol}
	res, err := ex.Run(context.Background(), sandbox.Request{Runtime: runtime, Source: source})
	if err != nil {
		t.Fatal(err)
	}
	if res.Violation != sandbox.ViolationTimeout {
		t.Fatalf("expected timeout, got %+v", res)
	}
}

func lookPython3() bool {
	_, err := exec.LookPath("python3")
	return err == nil
}
