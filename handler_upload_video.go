package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

const (
	// Limit upload size to 1gb
	maxUploadSize = 1 << 30
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	videoIDKey := make([]byte, 32)
	rand.Read(videoIDKey)
	videoIDHex := hex.EncodeToString(videoIDKey)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}
	fmt.Println("uploading video to S3", videoID, "by user", userID)
	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to query db", err)
		return
	}
	if dbVideo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "user does not own this video", err)
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()
	//Get media type from header
	contentType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "No or incorrect media type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "incorrect media type", nil)
		return
	}
	tempF, err := os.CreateTemp("", "tubely-upload-*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to create temp file", err)
		return
	}
	defer os.Remove(tempF.Name()) //clean up
	defer tempF.Close()           //close file
	//Copy data to temporary file
	if _, err := (io.Copy(tempF, file)); err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to copy data to temp file", err)
		return
	}
	//Set the file pointer back to 0 so we can re-read the data
	_, err = tempF.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to set file back to 0", err)
		return
	}
	fastStart, err := processVideoForFastStart(tempF.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to process video for fastStart", err)
		return
	}
	fastStartFile, err := os.Open(fastStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to open file", err)
		return
	}
	defer os.Remove(fastStartFile.Name())
	defer fastStartFile.Close()
	aspectRatio, err := getVideoAspectRatio(fastStartFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to determine aspect ratio", err)
		return
	}
	//declare videoPrefix
	var videoPrefix string
	switch aspectRatio {
	case "16:9":
		videoPrefix = "landscape/"
	case "9:16":
		videoPrefix = "portrait/"
	default:
		videoPrefix = "other/"
	}
	videoIDS3Value := videoPrefix + videoIDHex + ".mp4"
	//Put object into S3 bucket
	_, err = cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(videoIDS3Value),
		Body:        fastStartFile,
		ContentType: aws.String(mediaType),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to upload to S3", err)
		return
	}
	NewVideoURL := "https://" + cfg.s3Bucket + ".s3." + cfg.s3Region + ".amazonaws.com/" + videoIDS3Value
	dbVideo.VideoURL = &NewVideoURL
	cfg.db.UpdateVideo(dbVideo)

}
