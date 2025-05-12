package main

import (
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Vernacular-ai/godub"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func splitAudio(file multipart.File) ([]string, error) {
	tmpFilePath, err := saveTempFile(file)
	if err != nil {
		return nil, err
	}

	if !needsSplitting(tmpFilePath) {
		return []string{tmpFilePath}, nil
	}

	return splitAudioBySilence(tmpFilePath)
}

func saveTempFile(file multipart.File) (string, error) {
	filename := generateRandomFileName()
	tmpFilePath := filepath.Join(os.TempDir(), filename)
	err := saveFile(file, tmpFilePath)
	return tmpFilePath, err
}

func needsSplitting(filePath string) bool {
	fi, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	size := fi.Size()
	return size >= int64(24*1024*1024)
}

func splitAudioBySilence(tmpFilePath string) ([]string, error) {
	totalDuration, err := getAdjustedDuration(tmpFilePath)
	if err != nil {
		return nil, err
	}

	silenceTimestamps, err := getSilenceTimestamps(tmpFilePath)
	if err != nil {
		return nil, err
	}

	chunkPaths := []string{}
	startTime := 0 * time.Second
	targetTime := 10 * time.Minute
	searchRange := 2 * time.Minute

	for startTime < totalDuration {
		splitTime, err := findSplitTime(startTime, targetTime, searchRange, totalDuration, silenceTimestamps)
		if err != nil {
			return nil, err
		}

		chunkPath, err := createChunk(tmpFilePath, startTime, splitTime)
		if err != nil {
			return nil, err
		}

		chunkPaths = append(chunkPaths, chunkPath)
		startTime = splitTime
	}

	return chunkPaths, nil
}

func getAdjustedDuration(filePath string) (time.Duration, error) {
	duration, err := getAudioDuration(filePath)
	if err != nil {
		return 0, err
	}
	return duration - 1*time.Second, nil // Remove 1 second to avoid the last chunk being empty
}

func createChunk(tmpFilePath string, startTime, splitTime time.Duration) (string, error) {
	chunkPath := filepath.Join(os.TempDir(), fmt.Sprintf("chunk-%d-%d-%d.mp3", time.Now().UnixNano(), startTime, splitTime))
	err := splitMp3At(tmpFilePath, chunkPath, startTime, splitTime)
	return chunkPath, err
}

// Generate a random file name for the uploaded audio
func generateRandomFileName() string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("uploaded_audio_%d.mp3", timestamp)
}

// Find the split time based on silence timestamps or target time
func findSplitTime(startTime time.Duration, targetTime time.Duration, searchRange time.Duration, totalDuration time.Duration, silenceTimestamps []time.Duration) (time.Duration, error) {
	var splitTime time.Duration
	foundSplit := false

	// Find the closest silence timestamp to the target time
	for _, silenceTime := range silenceTimestamps {
		if silenceTime >= startTime+targetTime-searchRange && silenceTime <= startTime+targetTime+searchRange {
			splitTime = silenceTime
			fmt.Println("Found silence at", splitTime)
			foundSplit = true
			break
		}
	}

	if !foundSplit {
		splitTime = startTime + targetTime
	}

	if splitTime >= totalDuration {
		splitTime = totalDuration
	}

	return splitTime, nil
}

func getAudioDuration(inputFilePath string) (time.Duration, error) {
	audioSegment, err := godub.NewLoader().Load(inputFilePath)
	if err != nil {
		return 0, err
	}

	return audioSegment.Duration(), nil
}

func splitMp3At(inputFilePath string, outputFilePath string, startTime time.Duration, endTime time.Duration) error {
	// like "00:09:59"
	startTimeString := fmt.Sprintf("%02d:%02d:%02d", int(startTime.Hours()), int(startTime.Minutes())%60, int(startTime.Seconds())%60)
	endTimeString := fmt.Sprintf("%02d:%02d:%02d", int(endTime.Hours()), int(endTime.Minutes())%60, int(endTime.Seconds())%60)

	args := ffmpeg.KwArgs{}

	if startTime > 0 {
		args["ss"] = startTimeString
	}
	if endTime > 0 {
		args["to"] = endTimeString
	}

	args["acodec"] = "copy"
	args["y"] = "" // overwrite output file if it exists
	err := ffmpeg.Input(inputFilePath).Output(outputFilePath, args).Run()
	if err != nil {
		return err
	}

	return nil
}

func getSilenceTimestamps(inputFilePath string) ([]time.Duration, error) {
	silenceArgs := ffmpeg.KwArgs{
		"af": "silencedetect=d=0.4",
		"f":  "null",
	}
	// io.Writer to capture the output
	stdErrWriter := &strings.Builder{}
	err := ffmpeg.Input(inputFilePath).Output("-", silenceArgs).WithErrorOutput(stdErrWriter).Run()

	if err != nil {
		fmt.Println(stdErrWriter.String())
		return nil, err
	}

	return parseSilenceTimestamps(stdErrWriter.String())
}

func parseSilenceTimestamps(silenceOutput string) ([]time.Duration, error) {
	lines := strings.Split(silenceOutput, "\n")
	silenceTimestamps := []time.Duration{}

	for _, line := range lines {
		if strings.Contains(line, "silence_start") {
			line = strings.TrimSpace(line)
			parts := strings.Split(line, " ")
			for i, part := range parts {
				if part == "silence_start:" {
					if i+1 < len(parts) {
						silenceTime, err := strconv.ParseFloat(parts[i+1], 64)
						if err == nil {
							silenceTimestamps = append(silenceTimestamps, time.Duration(silenceTime*float64(time.Second)))
						}
					}
					break
				}
			}
		}
	}

	return silenceTimestamps, nil
}
