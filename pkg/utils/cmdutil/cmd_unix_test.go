//go:build unix

package cmdutil

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartReturnsWithoutWaitingAndWaitWaits(t *testing.T) {
	c := New("sh", []string{"-c", "sleep 0.3"})
	begin := time.Now()
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(begin); took > 200*time.Millisecond {
		t.Fatalf("Start took %s: it waited for the command", took)
	}
	if err := c.Wait(); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(begin); took < 300*time.Millisecond {
		t.Fatalf("Wait returned after %s, before the command ended", took)
	}
	if code := c.ExitCode(); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
}

func TestRunCollectsTheOutputWhenNoWriterIsGiven(t *testing.T) {
	c := New("sh", []string{"-c", "echo out; echo err >&2"})
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}
	if out, errOut := c.Output(), c.Error(); out != "out\n" || errOut != "err\n" {
		t.Fatalf("output = %q, error = %q", out, errOut)
	}
}

func TestAWriterTakesTheStreamInPlaceOfTheBuffer(t *testing.T) {
	var out, errOut strings.Builder
	c := New("sh", []string{"-c", "echo out; echo err >&2"}, WithStdout(&out), WithStderr(&errOut))
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "out\n" || errOut.String() != "err\n" {
		t.Fatalf("writers got %q and %q", out.String(), errOut.String())
	}
	if c.Output() != "" || c.Error() != "" {
		t.Fatalf("buffered %q and %q as well", c.Output(), c.Error())
	}
}

func TestRunReportsTheExitStatus(t *testing.T) {
	c := New("sh", []string{"-c", "exit 3"})
	err := c.Run()
	if err == nil || err.Error() != "exit status 3" {
		t.Fatalf("Run = %v, want exit status 3", err)
	}
	if code := c.ExitCode(); code != 3 {
		t.Fatalf("exit code = %d", code)
	}
}

func TestTerminateReachesTheWholeGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	c := New("sh", []string{"-c", childScript, "sh", pidFile}, WithProcessGroup())
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	child := readPid(t, pidFile)
	if err := c.Terminate(); err != nil {
		t.Fatal(err)
	}
	if err := c.Wait(); err == nil {
		t.Fatal("a terminated command must not report success")
	}
	assertGone(t, child)
}

func TestKillEndsWhatIgnoresTheTerm(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	c := New("sh", []string{"-c", "trap '' TERM; " + stubbornChild + " & wait", "sh", pidFile}, WithProcessGroup())
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	child := readPid(t, pidFile)
	if err := c.Terminate(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if gone(child) {
		t.Fatal("the child ignoring SIGTERM is gone before the kill")
	}
	if err := c.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := c.Wait(); err == nil {
		t.Fatal("a killed command must not report success")
	}
	assertGone(t, child)
}

func TestSignalingACommandNotStartedIsAnError(t *testing.T) {
	c := New("sh", []string{"-c", "true"})
	if err := c.Terminate(); err == nil {
		t.Fatal("Terminate before Start = nil")
	}
	if err := c.Kill(); err == nil {
		t.Fatal("Kill before Start = nil")
	}
}
