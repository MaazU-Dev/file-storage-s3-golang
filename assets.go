package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return "", errors.New("Unable to run the command")
	}
	var result struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		return "", errors.New("Unable to parse into the JSON")
	}
	width := result.Streams[0].Width
	height := result.Streams[0].Height
	if height == 0 {
		return "", errors.New("height cannot be zero")
	}
	aspectRatio := float64(width) / float64(height)
	if aspectRatio > 1.7 && aspectRatio < 1.8 {
		return "16:9", nil
	} else if aspectRatio > 0.5 && aspectRatio < 0.6 {
		return "9:16", nil
	} else {
		return "other", nil
	}
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := fmt.Sprintf("%s.processing", filePath)

	cmd := exec.Command(
		"ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)

	var buf bytes.Buffer
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return "", errors.New("Failed to run the command")
	}

	return outputFilePath, nil
}
