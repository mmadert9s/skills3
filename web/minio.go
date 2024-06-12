package main

import (
	"fmt"
	"io"
	"log"
	"net/url"

	"github.com/google/uuid"
	"github.com/minio/minio-go"
)

func uploadToS3(mc *minio.Client, filename string, body io.Reader, size int64) (string, error) {
	log.Printf("Uploading file to s3...")

	if filename == "" {
		uuid := uuid.NewString()
		filename = fmt.Sprintf("uploaded-%s", uuid)
	}

	n, err := mc.PutObject(bucketName, filename, body, size, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}

	objectUrl := url.URL{
		Scheme: "http",
		Host:   minioEndpoint,
		Path:   "/" + bucketName + "/" + filename,
	}

	log.Printf("Wrote %d bytes uploading object %v", n, filename)
	return objectUrl.String(), nil
}

func initMinio() *minio.Client {
	log.Print("Initializing minio...")

	minioClient, err := minio.New(minioEndpoint, accessKey, secretKey, false)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Created minio client")

	bucketExists, err := minioClient.BucketExists(bucketName)
	if err != nil {
		log.Fatal(err)
	}

	if !bucketExists {
		log.Print("Bucket doesn't exist, creating...")
		err = minioClient.MakeBucket(bucketName, "")
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Print("Minio initialized")
	return minioClient
}
