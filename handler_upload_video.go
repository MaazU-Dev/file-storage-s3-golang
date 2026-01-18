package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)
	videoIdString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIdString)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to parse UUID", err)
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

	metaData, err := cfg.db.GetVideo(videoID)

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unable to parse file", err)
		return
	}

	if metaData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unable to parse file", err)
		return
	}

	videoFile, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to parse file", err)
		return
	}
	defer videoFile.Close()

	mediaType := header.Header.Get("Content-Type")

	mediaType, _, err = mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to parse mime type", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusUnauthorized, "Please upload a MP4 Video", err)
		return
	}

	file, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to recreate temp file from OS", err)
		return
	}
	defer os.Remove(file.Name())
	defer file.Close()

	_, err = io.Copy(file, videoFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to copy file", err)
		return
	}

	file.Seek(0, io.SeekStart)

	aspectRatio, err := getVideoAspectRatio(file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get aspect ratio", err)
		return
	}
	processedFilePath, err := processVideoForFastStart(file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "File path not processed", err)
		return
	}
	defer os.Remove(processedFilePath)

	processedFile, err := os.Open(processedFilePath)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to open processed file", err)
		return
	}
	defer processedFile.Close()

	if err = os.Remove(file.Name()); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to delete existing file", err)
		return
	}

	var aspectRatioPrefix string
	switch aspectRatio {
	case "16:9":
		aspectRatioPrefix = "landscape"
	case "9:16":
		aspectRatioPrefix = "portrait"
	default:
		aspectRatioPrefix = "other"
	}

	key := make([]byte, 32)
	rand.Read(key)
	fileName := base64.RawURLEncoding.EncodeToString(key)
	fileExtension := strings.Split(mediaType, "/")[1]
	fileName = fmt.Sprintf("%s/%s.%s", aspectRatioPrefix, fileName, fileExtension)

	ctx := context.TODO()
	_, err = cfg.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(fileName),
		Body:        processedFile,
		ContentType: aws.String(mediaType),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to upload video to S3", err)
		return
	}
	// videoUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fileName)
	// videoUrl := fmt.Sprintf("%s,%s", cfg.s3Bucket, fileName)
	videoUrl := fmt.Sprintf("%s/%s", cfg.s3CfDistribution, fileName)
	fmt.Println("_+_+_+_+_+_+_+_+_+_+")
	fmt.Println("cfg.s3CfDistribution", cfg.s3CfDistribution)
	fmt.Println("videoUrl", videoUrl)
	fmt.Println("_+_+_+_+_+_+_+_+_+_+")

	metaData.VideoURL = &videoUrl

	err = cfg.db.UpdateVideo(metaData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Meta Data not updated", err)
		return
	}

	// metaData, err = cfg.dbVideoToSignedVideo(metaData)

	// if err != nil {
	// 	respondWithError(w, http.StatusInternalServerError, "Unable to get Presigned URL", err)
	// 	return
	// }

	respondWithJSON(w, http.StatusOK, metaData)
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)
	ctx := context.TODO()
	val, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", errors.New("Failed")
	}
	return val.URL, nil

}
