package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func uploadToS3(mc *minio.Client, filename string, body io.Reader, size int64) (string, error) {
	log.Printf("Uploading file to s3...")

	if filename == "" {
		uuid := uuid.NewString()
		filename = fmt.Sprintf("uploaded-%s", uuid)
	}

	info, err := mc.PutObject(context.Background(), bucketName, filename, body, size, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}

	objectUrl := url.URL{
		Scheme: "http",
		Host:   minioEndpoint,
		Path:   "/" + info.Bucket + "/" + info.Key,
	}

	log.Printf("Uploaded object %v", info.Key)
	return objectUrl.String(), nil
}

func initMinio() *minio.Client {
	log.Print("Initializing minio...")

	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})

	if err != nil {
		log.Fatal(err)
	}

	log.Print("Created minio client")

	bucketExists, err := minioClient.BucketExists(context.Background(), bucketName)
	if err != nil {
		log.Fatal(err)
	}

	if !bucketExists {
		log.Print("Bucket doesn't exist, creating...")
		err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Print("Minio initialized")
	return minioClient
}
