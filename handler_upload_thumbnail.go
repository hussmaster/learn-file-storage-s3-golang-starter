package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDKey := make([]byte, 32)
	rand.Read(videoIDKey)
	videoIDbase64 := base64.RawURLEncoding.EncodeToString(videoIDKey)
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)

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
	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	//Get media type from header
	contentType := header.Header["Content-Type"]
	mediaType, _, err := mime.ParseMediaType(contentType[0])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "No content type", err)
	}
	defer file.Close()

	//Get video metadata from database using the videoID
	videoMetadata, err := cfg.db.GetVideo(uuid.UUID(videoID))
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized user", err)
		return
	}

	//Declare extension string
	var extension string
	switch mediaType {
	case "image/jpg":
		extension = ".jpg"
	case "image/png":
		extension = ".png"
	default:
		respondWithError(w, http.StatusBadRequest, "invalid file", err)
		return
	}
	//Create new file with videoID for a unique path
	newFilePath := filepath.Join(cfg.assetsRoot, videoIDbase64+extension)
	f, err := os.Create(newFilePath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to create file", err)
		return
	}
	if _, err := io.Copy(f, file); err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to upload file", err)
		return
	}

	//Set new temp var for thumbnail url that is addressable
	newThumbnailURL := "http://192.168.1.141:" + cfg.port + "/assets/" + videoIDbase64 + extension
	log.Println(newThumbnailURL)
	//Set local struct to new thumbnail URL
	videoMetadata.ThumbnailURL = &newThumbnailURL
	//Update database with updated thumbnail URL
	cfg.db.UpdateVideo(videoMetadata)

	//Pass updated struct to repsond with json
	respondWithJSON(w, http.StatusOK, videoMetadata)
}
