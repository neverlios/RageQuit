package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// CacheDir returns the path to the RageQuit cache directory.
func CacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "RageQuit")
}

// PidPath returns the path to the PID file.
func PidPath() string {
	return filepath.Join(CacheDir(), "ragequit.pid")
}

// LogPath returns the path to the daemon log file.
func LogPath() string {
	return filepath.Join(CacheDir(), "ragequit.log")
}

// WritePid writes the given PID to the PID file.
func WritePid(pid int) error {
	if err := os.MkdirAll(CacheDir(), 0755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	return os.WriteFile(PidPath(), []byte(strconv.Itoa(pid)), 0644)
}

// ReadPid reads the PID from the PID file.
func ReadPid() (int, error) {
	data, err := os.ReadFile(PidPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

// RemovePid removes the PID file.
func RemovePid() error {
	return os.Remove(PidPath())
}

// IsRunning checks if the daemon is running by verifying the PID file
// and checking if the process exists.
func IsRunning() (bool, int) {
	pid, err := ReadPid()
	if err != nil {
		return false, 0
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}
	// Signal 0 checks if process exists without sending a signal
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist, clean up stale PID file
		RemovePid()
		return false, 0
	}
	return true, pid
}

// Stop sends SIGTERM to the daemon and waits for it to exit.
func Stop() error {
	pid, err := ReadPid()
	if err != nil {
		return fmt.Errorf("reading pid: %w", err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process: %w", err)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		RemovePid()
		return nil
	}
	// Wait for process to exit (up to 5 seconds)
	for i := 0; i < 50; i++ {
		if err := process.Signal(syscall.Signal(0)); err != nil {
			// Process has exited
			RemovePid()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Force kill if still running
	process.Signal(syscall.SIGKILL)
	RemovePid()
	return nil
}
