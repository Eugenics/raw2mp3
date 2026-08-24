package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

const (
	wavDir    = "wav"
	mp3Dir    = "mp3"
	ffmpegDir = "ffmpeg"
	workers   = 4 // number of concurrent conversions
)

func ffmpegPath() string {
	// Look for ffmpeg in local ffmpeg/ directory first
	exe := "ffmpeg"
	if runtime.GOOS == "windows" {
		exe = "ffmpeg.exe"
	}
	localPath := filepath.Join(ffmpegDir, exe)
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}
	// Fallback to PATH
	return "ffmpeg"
}

func main() {
	// Ensure directories exist
	if err := os.MkdirAll(wavDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create wav dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(mp3Dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create mp3 dir: %v\n", err)
		os.Exit(1)
	}

	// Find all .wav files
	files, err := filepath.Glob(filepath.Join(wavDir, "*.wav"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to glob wav files: %v\n", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Println("No .wav files found in ./wav directory")
		return
	}

	fmt.Printf("Found %d .wav file(s)\n", len(files))

	// Worker pool
	jobs := make(chan string, len(files))
	results := make(chan string, len(files))
	var wg sync.WaitGroup

	// Start workers
	numWorkers := workers
	if numWorkers > runtime.NumCPU() {
		numWorkers = runtime.NumCPU()
	}
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for wavFile := range jobs {
				mp3File := convert(wavFile)
				results <- mp3File
			}
		}(i)
	}

	// Send jobs
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	// Wait for completion
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	success := 0
	failed := 0
	for r := range results {
		if r != "" {
			fmt.Printf("✓ %s\n", r)
			success++
		} else {
			failed++
		}
	}

	fmt.Printf("\nDone: %d converted, %d failed\n", success, failed)
}

func convert(wavPath string) string {
	base := filepath.Base(wavPath)
	name := base[:len(base)-4] // remove .wav
	mp3Path := filepath.Join(mp3Dir, name+".mp3")

	// ffmpeg -i input.wav -codec:a libmp3lame -qscale:a 2 output.mp3
	// -qscale:a 2 = high quality VBR (~190 kbps)
	cmd := []string{
		"-y",              // overwrite output
		"-i", wavPath,     // input
		"-codec:a", "libmp3lame",
		"-qscale:a", "2",  // quality (0-9, lower = better)
		mp3Path,
	}

	err := runFFmpeg(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Failed to convert %s: %v\n", wavPath, err)
		return ""
	}
	return mp3Path
}

func runFFmpeg(args []string) error {
	cmd := exec.Command(ffmpegPath(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}