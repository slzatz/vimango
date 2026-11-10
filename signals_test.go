package main

import (
	"bufio"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestHEICWorkerHelper is not a test: TestKilledProcessTakesItsHEICWorkerAlong
// runs this binary again with VIMANGO_HEIC_HELPER set, and this starts the
// real heic_worker, as the first HEIC image in a note does, and waits to be
// killed. "handled" installs the signal handler vimango installs; "bare" is
// vimango before it had one.
func TestHEICWorkerHelper(t *testing.T) {
	how, ok := os.LookupEnv("VIMANGO_HEIC_HELPER")
	if !ok {
		t.Skip("helper for TestKilledProcessTakesItsHEICWorkerAlong")
	}
	if how == "handled" {
		killHEICWorkerOnSignal()
	}
	if !createHEICDecoder().IsAvailable() {
		os.Stdout.WriteString("HEIC-UNAVAILABLE\n")
		return
	}
	os.Stdout.WriteString("HEIC-READY\n")
	time.Sleep(time.Minute)
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

// A vimango that is killed takes its heic_worker with it. vimango_hybrid
// stops a --render-html it no longer wants with SIGTERM, and a signal's
// default action runs neither the deferred ShutdownHEICDecoder nor anything
// else: the worker stayed, holding its memory until the machine restarted.
func TestKilledProcessTakesItsHEICWorkerAlong(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns processes")
	}
	if _, err := os.Stat("heic_worker"); err != nil {
		t.Skip("no heic_worker binary here (see CLAUDE.md, Building heic_worker)")
	}

	// run starts a vimango with a worker, kills it with SIGTERM and reports
	// the worker's pid and how the vimango ended.
	run := func(how string) (worker int, ended error) {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestHEICWorkerHelper$", "-test.v")
		cmd.Env = append(os.Environ(), "VIMANGO_HEIC_HELPER="+how)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		ready := false
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if strings.Contains(sc.Text(), "HEIC-UNAVAILABLE") {
				cmd.Process.Kill()
				cmd.Wait()
				t.Skip("heic_worker is here but does not start (wrong architecture?)")
			}
			if strings.Contains(sc.Text(), "HEIC-READY") {
				ready = true
				break
			}
		}
		if !ready {
			cmd.Process.Kill()
			cmd.Wait()
			t.Fatalf("%s: the helper never started its worker", how)
		}
		out, err := exec.Command("pgrep", "-P", strconv.Itoa(cmd.Process.Pid)).Output()
		if err != nil {
			cmd.Process.Kill()
			cmd.Wait()
			t.Fatalf("%s: the helper has no child process: %v", how, err)
		}
		worker, err = strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			cmd.Process.Kill()
			cmd.Wait()
			t.Fatalf("%s: expected one child, the worker; pgrep says %q", how, out)
		}
		t.Cleanup(func() { syscall.Kill(worker, syscall.SIGKILL) })

		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		go func() { // the pipe must drain for Wait to return
			for sc.Scan() {
			}
		}()
		return worker, cmd.Wait()
	}

	gone := func(pid int) bool {
		for range 30 {
			if !alive(pid) {
				return true
			}
			time.Sleep(100 * time.Millisecond)
		}
		return false
	}

	worker, ended := run("handled")
	if !gone(worker) {
		t.Errorf("the worker (pid %d) is still running three seconds after its vimango was killed", worker)
	}
	// and the host still sees a process that died of the signal it sent
	if ee, ok := ended.(*exec.ExitError); !ok || ee.Sys().(syscall.WaitStatus).Signal() != syscall.SIGTERM {
		t.Errorf("the killed vimango ended with %v, want death by SIGTERM", ended)
	}

	// the control: without the handler the worker is orphaned
	worker, _ = run("bare")
	time.Sleep(300 * time.Millisecond)
	if !alive(worker) {
		t.Errorf("the control is wrong: the worker died with a vimango that had no handler")
	}
}
