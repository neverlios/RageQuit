package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	accelerometer "github.com/taigrr/apple-silicon-accelerometer/sensor"
	"github.com/taigrr/apple-silicon-accelerometer/shm"
	"ragequit/daemon"
	"ragequit/detector"
	"ragequit/display"
)

// findImages returns all image files found in dir.
// Supports ~/ prefix for the home directory.
func findImages(dir string) ([]string, error) {
	if strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home dir: %w", err)
		}
		dir = filepath.Join(home, dir[2:])
	}

	var found []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp":
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", dir, err)
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no images found in %s", dir)
	}
	return found, nil
}

// findImage returns the path to the first image file found in dir.
// Supports ~/ prefix for the home directory.
func findImage(dir string) (string, error) {
	images, err := findImages(dir)
	if err != nil {
		return "", err
	}
	return images[0], nil
}

// runDaemon contains the main daemon logic.
func runDaemon(imageDir string, minAmplitude float64, cooldownMs int, isDaemon bool) error {
	// If running as daemon, manage PID file
	if isDaemon {
		if err := daemon.WritePid(os.Getpid()); err != nil {
			return fmt.Errorf("writing pid: %w", err)
		}
		defer daemon.RemovePid()
	}

	images, err := findImages(imageDir)
	if err != nil {
		return fmt.Errorf("finding images: %w", err)
	}
	fmt.Printf("ragequit: found %d image(s) in %s\n", len(images), imageDir)
	fmt.Printf("ragequit: sensitivity %.2fg, cooldown %dms\n", minAmplitude, cooldownMs)

	// Compile the Swift display binary on first run (one-time ~10s).
	if err := display.CompileIfNeeded(); err != nil {
		return fmt.Errorf("preparing display: %w", err)
	}

	// Create shared memory ring buffer for accelerometer data.
	accelRing, err := shm.CreateRing("/ragequit-accel")
	if err != nil {
		return fmt.Errorf("creating accel ring: %w", err)
	}
	defer accelRing.Close()
	defer accelRing.Unlink()

	det := detector.New(minAmplitude, time.Duration(cooldownMs)*time.Millisecond)

	// Sensor goroutine: reads IOKit HID → writes to accelRing.
	go func() {
		if err := accelerometer.Run(accelerometer.Config{
			AccelRing: accelRing,
		}); err != nil {
			log.Printf("sensor error: %v", err)
		}
	}()

	// Detection goroutine: polls accelRing → spawns display subprocess.
	go func() {
		var lastTotal uint64
		const maxBatch = 200
		for {
			samples, newTotal := accelRing.ReadNew(lastTotal, shm.AccelScale)
			lastTotal = newTotal
			if len(samples) > maxBatch {
				samples = samples[len(samples)-maxBatch:]
			}
			for _, s := range samples {
				if det.Check(s.X, s.Y, s.Z) {
					pick := images[rand.Intn(len(images))]
					fmt.Printf("ragequit: impact! showing %s\n", filepath.Base(pick))
					display.Show(pick)
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Block until Ctrl+C or SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\nragequit: shutting down.")
	return nil
}

func main() {
	var imageDir string
	var minAmplitude float64
	var cooldownMs int
	var isDaemon bool

	rootCmd := &cobra.Command{
		Use:   "ragequit",
		Short: "Show image on all displays when your Mac is spanked",
	}

	// run command - direct execution (foreground)
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run ragequit in foreground (blocking)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon(imageDir, minAmplitude, cooldownMs, isDaemon)
		},
	}
	runCmd.Flags().StringVarP(&imageDir, "image-dir", "i", "~/RageQuitImgs/", "Directory containing images to display")
	runCmd.Flags().Float64VarP(&minAmplitude, "min-amplitude", "a", 0.6, "Impact threshold in g-force deviation from 1g")
	runCmd.Flags().IntVarP(&cooldownMs, "cooldown", "c", 750, "Milliseconds before the image can be triggered again")
	runCmd.Flags().BoolVar(&isDaemon, "daemon", false, "Internal flag: run as daemon (writes PID file)")
	runCmd.Flags().MarkHidden("daemon")

	// start command - launch daemon in background
	var startImageDir string
	var startMinAmplitude float64
	var startCooldownMs int

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start ragequit daemon in background",
		RunE: func(cmd *cobra.Command, args []string) error {
			if running, pid := daemon.IsRunning(); running {
				return fmt.Errorf("ragequit is already running (pid %d)", pid)
			}

			// Build arguments for the run command
			runArgs := []string{"run", "--daemon"}
			if cmd.Flags().Changed("image-dir") {
				runArgs = append(runArgs, "--image-dir", startImageDir)
			}
			if cmd.Flags().Changed("min-amplitude") {
				runArgs = append(runArgs, "--min-amplitude", fmt.Sprintf("%f", startMinAmplitude))
			}
			if cmd.Flags().Changed("cooldown") {
				runArgs = append(runArgs, "--cooldown", fmt.Sprintf("%d", startCooldownMs))
			}

			// Get the executable path
			executable, err := os.Executable()
			if err != nil {
				return fmt.Errorf("getting executable path: %w", err)
			}

			// Open log file for daemon output
			logFile, err := os.OpenFile(daemon.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return fmt.Errorf("opening log file: %w", err)
			}

			// Start the daemon process
			daemonCmd := exec.Command(executable, runArgs...)
			daemonCmd.Stdout = logFile
			daemonCmd.Stderr = logFile
			daemonCmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid: true, // Create new process group
			}

			if err := daemonCmd.Start(); err != nil {
				logFile.Close()
				return fmt.Errorf("starting daemon: %w", err)
			}

			logFile.Close()
			fmt.Printf("ragequit: started (pid %d)\n", daemonCmd.Process.Pid)
			fmt.Printf("ragequit: logs at %s\n", daemon.LogPath())
			return nil
		},
	}
	startCmd.Flags().StringVarP(&startImageDir, "image-dir", "i", "~/RageQuitImgs/", "Directory containing images to display")
	startCmd.Flags().Float64VarP(&startMinAmplitude, "min-amplitude", "a", 0.6, "Impact threshold in g-force deviation from 1g")
	startCmd.Flags().IntVarP(&startCooldownMs, "cooldown", "c", 750, "Milliseconds before the image can be triggered again")

	// stop command
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the ragequit daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			running, pid := daemon.IsRunning()
			if !running {
				fmt.Println("ragequit: not running")
				return nil
			}
			if err := daemon.Stop(); err != nil {
				return fmt.Errorf("stopping daemon: %w", err)
			}
			fmt.Printf("ragequit: stopped (pid %d)\n", pid)
			return nil
		},
	}

	// status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Check if ragequit daemon is running",
		Run: func(cmd *cobra.Command, args []string) {
			if running, pid := daemon.IsRunning(); running {
				fmt.Printf("ragequit: running (pid %d)\n", pid)
			} else {
				fmt.Println("ragequit: not running")
			}
		},
	}

	rootCmd.AddCommand(runCmd, startCmd, stopCmd, statusCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
