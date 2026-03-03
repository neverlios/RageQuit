package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	accelerometer "github.com/taigrr/apple-silicon-accelerometer/sensor"
	"github.com/taigrr/apple-silicon-accelerometer/shm"
	"spankimg/detector"
	"spankimg/display"
)

// findImage returns the path to the first image file found in dir.
// Supports ~/ prefix for the home directory.
func findImage(dir string) (string, error) {
	if strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home dir: %w", err)
		}
		dir = filepath.Join(home, dir[2:])
	}

	var found string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp":
			found = path
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scanning %s: %w", dir, err)
	}
	if found == "" {
		return "", fmt.Errorf("no image found in %s", dir)
	}
	return found, nil
}

func main() {
	// Lock main goroutine to the main OS thread — required for [NSApp run].
	runtime.LockOSThread()

	var imageDir string
	var minAmplitude float64
	var cooldownMs int

	cmd := &cobra.Command{
		Use:   "spankimg",
		Short: "Show image on all displays when your Mac is spanked",
		RunE: func(cmd *cobra.Command, args []string) error {
			imagePath, err := findImage(imageDir)
			if err != nil {
				return fmt.Errorf("finding image: %w", err)
			}
			fmt.Printf("spankimg: using image %s\n", imagePath)
			fmt.Printf("spankimg: sensitivity %.2fg, cooldown %dms\n", minAmplitude, cooldownMs)

			// Create shared memory ring buffer for accelerometer data.
			accelRing, err := shm.CreateRing("/spankimg-accel")
			if err != nil {
				return fmt.Errorf("creating accel ring: %w", err)
			}
			defer accelRing.Close()
			defer accelRing.Unlink()

			det := detector.New(minAmplitude, time.Duration(cooldownMs)*time.Millisecond)

			display.Init()

			// Sensor goroutine: reads from IOKit HID and writes to accelRing.
			// sensor.Run() locks itself to an OS thread internally.
			go func() {
				if err := accelerometer.Run(accelerometer.Config{
					AccelRing: accelRing,
				}); err != nil {
					log.Printf("sensor error: %v", err)
				}
			}()

			// Detection goroutine: polls accelRing and triggers display.
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
							fmt.Println("spankimg: impact detected!")
							display.Show(imagePath)
						}
					}
					time.Sleep(10 * time.Millisecond)
				}
			}()

			// RunLoop blocks forever on the main thread — must be last.
			display.RunLoop()
			return nil
		},
	}

	cmd.Flags().StringVarP(&imageDir, "image-dir", "i", "~/spankimg/", "Directory containing the image to display")
	cmd.Flags().Float64VarP(&minAmplitude, "min-amplitude", "a", 0.3, "Impact threshold in g-force deviation from 1g")
	cmd.Flags().IntVarP(&cooldownMs, "cooldown", "c", 750, "Milliseconds before the image can be triggered again")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
