package exec_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/myserver/daemon/internal/exec"
)

func TestRun_StreamsStdout(t *testing.T) {
	eng := exec.NewEngine()
	defer eng.KillAll()

	var lines []string
	var stderr []string
	var done bool
	res, err := eng.Run(context.Background(), exec.Options{
		Command: `printf "alpha\nbeta\n"`,
		Timeout: 5 * time.Second,
	}, func(ev exec.StreamEvent) {
		switch ev.Stream {
		case "stdout":
			lines = append(lines, ev.Data)
		case "stderr":
			stderr = append(stderr, ev.Data)
		}
		if ev.Done {
			done = true
		}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit=%d stderr=%v", res.ExitCode, stderr)
	}
	if !done {
		t.Fatal("done event not received")
	}
	if got := strings.Join(lines, "|"); got != "alpha|beta" {
		t.Fatalf("stdout=%q want alpha|beta stderr=%v", got, stderr)
	}
}

func TestRun_RespectsTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in -short")
	}
	eng := exec.NewEngine()
	defer eng.KillAll()

	start := time.Now()
	_, err := eng.Run(context.Background(), exec.Options{
		Command: `sleep 5`,
		Timeout: 500 * time.Millisecond,
	}, func(exec.StreamEvent) {})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("timeout took too long: %s", elapsed)
	}
}

func TestRun_ExitCodePropagates(t *testing.T) {
	eng := exec.NewEngine()
	defer eng.KillAll()

	var done *exec.StreamEvent
	res, err := eng.Run(context.Background(), exec.Options{
		Command: `exit 7`,
		Timeout: 3 * time.Second,
	}, func(ev exec.StreamEvent) {
		if ev.Done {
			cp := ev
			done = &cp
		}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.ExitCode != 7 {
		t.Fatalf("exit=%d want 7", res.ExitCode)
	}
	if done == nil || done.ExitCode != 7 {
		t.Fatalf("done event missing or wrong code: %+v", done)
	}
}
