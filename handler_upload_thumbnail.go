package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	const maxMemory = 10 << 20

	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to parse file", err)
		return
	}

	defer file.Close()

	mediaType := header.Header.Get("Content-Type")

	if mediaType == "" {
		respondWithError(w, http.StatusInternalServerError, "Missing content type for thumbnail", err)
		return
	}

	metaData, err := cfg.db.GetVideo(videoID)

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unable to parse file", err)
		return
	}

	fileExtension := strings.Split(mediaType, "/")[1]
	videoFileName := fmt.Sprintf("%s.%s", videoID, fileExtension)
	filePath := filepath.Join(cfg.assetsRoot, videoFileName)
	newFile, err := os.Create(filePath)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create new file", err)
		return
	}

	defer newFile.Close()

	_, err = io.Copy(newFile, file)
	thumbnailURL := fmt.Sprintf("/assets/%s", videoFileName)
	metaData.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(metaData)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video meta data", err)
		return
	}

	respondWithJSON(w, http.StatusOK, metaData)
}
