package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	WavDir    string `yaml:"wav_dir"`
	Mp3Dir    string `yaml:"mp3_dir"`
	FFmpegDir string `yaml:"ffmpeg_dir"`
	Workers   int    `yaml:"workers"`
	Quality   int    `yaml:"quality"`
	Bitrate   string `yaml:"bitrate"`
	Codec     string `yaml:"codec"`
	Overwrite bool   `yaml:"overwrite"`
	Verbose   bool   `yaml:"verbose"`
}

var cfg Config

func LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		// Return defaults if config file doesn't exist
		if os.IsNotExist(err) {
			cfg = Config{
				WavDir:    "wav",
				Mp3Dir:    "mp3",
				FFmpegDir: "ffmpeg",
				Workers:   4,
				Quality:   2,
				Bitrate:   "",
				Codec:     "libmp3lame",
				Overwrite: true,
				Verbose:   false,
			}
			return nil
		}
		return err
	}
	return yaml.Unmarshal(data, &cfg)
}

func GetConfig() Config {
	return cfg
}