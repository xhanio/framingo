//go:build unix

package cmdutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const childScript = `sleep 60 & echo $! > "$1"; wait`

func readPid(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil && strings.HasSuffix(string(b), "\n") {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child pid never written")
	return 0
}

// an unreaped zombie counts as gone
func gone(pid int) bool {
	if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
		return true
	}
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	// the state follows the parenthesized command name: "<pid> (<comm>) <state> ..."
	rest := string(stat[strings.LastIndexByte(string(stat), ')')+1:])
	return strings.HasPrefix(strings.TrimSpace(rest), "Z")
}

func assertGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if gone(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child %d outlived its canceled parent", pid)
}

func TestUseProcessGroupStopsChildrenOnCancel(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := exec.CommandContext(ctx, "sh", "-c", childScript, "sh", pidFile)
	UseProcessGroup(p)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	child := readPid(t, pidFile)
	cancel()
	if err := p.Wait(); err == nil {
		t.Fatal("a command stopped by cancel must not report success")
	}
	assertGone(t, child)
}

func TestWithProcessGroupStopsChildrenOnCancel(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New("sh", []string{"-c", childScript, "sh", pidFile}, WithContext(ctx), WithProcessGroup())
	done := make(chan error, 1)
	go func() { done <- c.Start() }()
	child := readPid(t, pidFile)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a command stopped by cancel must not report success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("command did not return after cancel")
	}
	assertGone(t, child)
}
