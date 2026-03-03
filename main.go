package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	accelerometer "github.com/taigrr/apple-silicon-accelerometer/sensor"
	"github.com/taigrr/apple-silicon-accelerometer/shm"
	"spankimg/detector"
	"spankimg/display"
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

func main() {
	var imageDir string
	var minAmplitude float64
	var cooldownMs int

	cmd := &cobra.Command{
		Use:   "spankimg",
		Short: "Show image on all displays when your Mac is spanked",
		RunE: func(cmd *cobra.Command, args []string) error {
			images, err := findImages(imageDir)
			if err != nil {
				return fmt.Errorf("finding images: %w", err)
			}
			fmt.Printf("spankimg: found %d image(s) in %s\n", len(images), imageDir)
			fmt.Printf("spankimg: sensitivity %.2fg, cooldown %dms\n", minAmplitude, cooldownMs)

			// Compile the Swift display binary on first run (one-time ~10s).
			if err := display.CompileIfNeeded(); err != nil {
				return fmt.Errorf("preparing display: %w", err)
			}

			// Create shared memory ring buffer for accelerometer data.
			accelRing, err := shm.CreateRing("/spankimg-accel")
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
							fmt.Printf("spankimg: impact! showing %s\n", filepath.Base(pick))
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
			fmt.Println("\nspankimg: shutting down.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&imageDir, "image-dir", "i", "~/spankimg/", "Directory containing the image to display")
	cmd.Flags().Float64VarP(&minAmplitude, "min-amplitude", "a", 0.6, "Impact threshold in g-force deviation from 1g")
	cmd.Flags().IntVarP(&cooldownMs, "cooldown", "c", 750, "Milliseconds before the image can be triggered again")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
