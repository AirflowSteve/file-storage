package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't get video id", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid token", err)
		return
	}

	usrID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid token", err)
		return
	}

	videoMD, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "nah", err)
		return
	}
	if videoMD.UserID != usrID {
		respondWithError(w, http.StatusUnauthorized, "assecc denied", err)
		return
	}

	//need to get formfile and so on

	const uploadLimit = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "couldn't parse the file", err)
		return
	}

	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type, only MP4 is allowed", nil)
		return
	}

	tempVid, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't create a temp file", err)
		return
	}

	defer os.Remove(tempVid.Name())
	defer tempVid.Close()

	_, err = io.Copy(tempVid, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error saving file", err)
		return
	}

	if _, err := tempVid.Seek(0, io.SeekStart); err != nil {
		respondWithError(w, http.StatusInternalServerError, "nah", err)
		return
	}

	processedFile, err := processVideoForFastStart(tempVid.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't process the video for faststart", err)
		return
	}
	defer os.Remove(processedFile)

	procVid, err := os.Open(processedFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't open processed file", err)
		return
	}
	defer procVid.Close()

	prefix, err := getVideoAspectRatio(tempVid.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't get the aspect ratio of a video", err)
		return
	}

	key := path.Join(prefix, getAssetPath(mediaType))

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        procVid,
		ContentType: aws.String(mediaType),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error uploading file to S3", err)
		return
	}

	url := fmt.Sprintf("%v/%v", cfg.s3CfDistribution, key)
	videoMD.VideoURL = &url
	err = cfg.db.UpdateVideo(videoMD)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "nope", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMD)

}
