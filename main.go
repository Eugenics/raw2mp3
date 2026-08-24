package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
)

var (
	quality    *int
	bitrate    *string
	codec      *string
	threads    *int
	overwrite  *bool
	verbose    *bool
	configPath string
)

func main() {
	// First pass: parse -config manually from os.Args
	configPath := "config.yaml"
	for i, arg := range os.Args[1:] {
		if arg == "-config" && i+1 < len(os.Args)-1 {
			configPath = os.Args[i+2]
			break
		}
	}

	// Load config from specified file
	if err := LoadConfig(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	cfg := GetConfig()

	// Define all flags with config values as defaults
	quality = flag.Int("q", cfg.Quality, "VBR качество (0-9, меньше = лучше)")
	bitrate = flag.String("b", cfg.Bitrate, "CBR битрейт (например, 128k, 320k)")
	codec = flag.String("c", cfg.Codec, "Аудио кодек")
	threads = flag.Int("t", cfg.Workers, "Количество потоков (0 = авто)")
	overwrite = flag.Bool("y", cfg.Overwrite, "Перезаписывать выходные файлы")
	verbose = flag.Bool("v", cfg.Verbose, "Подробный вывод")
	flag.String("config", configPath, "Path to config file")

	// Parse all flags
	flag.Parse()

	// Ensure directories exist
	if err := os.MkdirAll(cfg.WavDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create wav dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.Mp3Dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create mp3 dir: %v\n", err)
		os.Exit(1)
	}

	// Find all .wav files
	files, err := filepath.Glob(filepath.Join(cfg.WavDir, "*.wav"))
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
	numWorkers := cfg.Workers
	if *threads > 0 {
		numWorkers = *threads
	}
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

func ffmpegPath() string {
	// Look for ffmpeg in local ffmpeg/ directory first
	exe := "ffmpeg"
	if runtime.GOOS == "windows" {
		exe = "ffmpeg.exe"
	}
	cfg := GetConfig()
	localPath := filepath.Join(cfg.FFmpegDir, exe)
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}
	// Fallback to PATH
	return "ffmpeg"
}

func convert(wavPath string) string {
	cfg := GetConfig()
	base := filepath.Base(wavPath)
	name := base[:len(base)-4] // remove .wav
	mp3Path := filepath.Join(cfg.Mp3Dir, name+".mp3")

	// Build ffmpeg command with flags
	cmd := []string{
		"-i", wavPath, // input
		"-codec:a", *codec,
	}

	// Quality settings: prefer CBR bitrate if specified, otherwise VBR quality
	if *bitrate != "" {
		cmd = append(cmd, "-b:a", *bitrate)
	} else {
		cmd = append(cmd, "-qscale:a", strconv.Itoa(*quality))
	}

	// Overwrite flag
	if *overwrite {
		cmd = append([]string{"-y"}, cmd...)
	}

	cmd = append(cmd, mp3Path)

	if *verbose {
		fmt.Printf("Running: %s %v\n", ffmpegPath(), cmd)
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