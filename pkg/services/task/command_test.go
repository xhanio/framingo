package task

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/jobs"
)

// writes its pid to "$0" once it ignores SIGTERM, and so does the child it waits for
const stubbornCommand = `trap "" TERM; sh -c 'trap "" TERM; echo $$ > "$0"; exec sleep 60' "$0" & wait`

func readPid(t *testing.T, path string) int {
	t.Helper()
	var pid int
	eventually(t, "the pid to be written", func() bool {
		raw, err := os.ReadFile(path)
		if err != nil || !strings.HasSuffix(string(raw), "\n") {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		return true
	})
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	return pid
}

// an unreaped zombie counts as gone
func processGone(pid int) bool {
	if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
		return true
	}
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	rest := string(stat[strings.LastIndexByte(string(stat), ')')+1:])
	return strings.HasPrefix(strings.TrimSpace(rest), "Z")
}

func TestRemoveOfACommandWaitsOutItsGracePeriod(t *testing.T) {
	s := startedManager(t)
	pids := filepath.Join(t.TempDir(), "pid")
	c := jobs.Command{Argv: []string{"sh", "-c", stubbornCommand, pids}, Grace: 200 * time.Millisecond}
	tk := &entity.Task{Job: job.New("stubborn", c.Run), Manual: true}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	child := readPid(t, pids)
	begin := time.Now()
	assertRemoved(t, removeInBackground(s, context.Background(), tk))
	if took := time.Since(begin); took < 200*time.Millisecond || took > 2*time.Second {
		t.Fatalf("Remove's end came %s after it, want about the 200ms grace period", took)
	}
	eventually(t, "the child to be gone", func() bool { return processGone(child) })
}
