package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var buf bytes.Buffer
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffprobe error: %v", err)
	}

	type parameters struct {
		Streams []struct {
			Width  int `json:"width,omitempty"`
			Height int `json:"height,omitempty"`
		} `json:"streams"`
	}

	params := parameters{}

	err := json.Unmarshal(buf.Bytes(), &params)
	if err != nil {
		return "", fmt.Errorf("could not parse ffprobe output: %v", err)

	}

	if len(params.Streams) == 0 {
		return "", errors.New("no streams found")
	}

	w := params.Streams[0].Width
	h := params.Streams[0].Height
	ratio := ""
	if w == 16*h/9 {
		ratio = "landscape"
	} else if h == 16*w/9 {
		ratio = "portrait"
	} else {
		ratio = "other"
	}

	return ratio, nil
}

func processVideoForFastStart(filePath string) (string, error) {
	outputPath := filePath + ".processing"
	var stderr bytes.Buffer
	cmd := exec.Command(
		"ffmpeg",
		"-i",
		filePath,
		"-c",
		"copy",
		"-movflags",
		"faststart",
		"-f",
		"mp4",
		outputPath,
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return outputPath, nil
}
