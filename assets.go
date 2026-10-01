package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
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
	//Video properties struct
	type videoProp struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	ffProbe := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var b bytes.Buffer //variable to capture output
	//Output studout to pointer for bytes Buffer
	ffProbe.Stdout = &b
	err := ffProbe.Run()
	if err != nil {
		log.Printf("can't run command: %v", err)
		return "", err
	}
	var jsonBody videoProp
	//Unmarshal into jsonBody videoProperties struct
	err = json.Unmarshal(b.Bytes(), &jsonBody)
	if err != nil {
		log.Printf("can't unmarshal json to struct: %v", err)
		return "", err
	}

	vWidthLand := jsonBody.Streams[0].Width * 9
	vHeightLand := jsonBody.Streams[0].Height * 16
	vWidthPort := jsonBody.Streams[0].Width * 16
	vHeightPort := jsonBody.Streams[0].Height * 9
	landscapeDelta := ratiowithinTenPercent(float64(vWidthLand), float64(vHeightLand))
	portraitDelta := ratiowithinTenPercent(float64(vWidthPort), float64(vHeightPort))
	if landscapeDelta == true && portraitDelta == false {
		return "16:9", nil
	} else if landscapeDelta == false && portraitDelta == true {
		return "9:16", nil
	} else if landscapeDelta == true && portraitDelta == true {
		return "other", nil
	} else {
		return "other", nil
	}
}

// Function to check for aspect ratio
func ratiowithinTenPercent(num, target float64) bool {
	delta := math.Abs(target) * 0.10
	return math.Abs(num-target) <= delta
}

// Function to move the moov atom to the start of the file
func processVideoForFastStart(filePath string) (string, error) {
	tempPath := filePath + ".processing"
	moovCmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", tempPath)
	err := moovCmd.Run()
	if err != nil {
		log.Printf("unable to process file: %v", err)
		return "", err
	}
	return tempPath, nil
}
