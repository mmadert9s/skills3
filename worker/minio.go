package main

import (
	"context"
	"log"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Assume bucket already exists.
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

	bucketExists, err := minioClient.BucketExists(context.Background(), outputBucketName)
	if err != nil {
		log.Fatal(err)
	}

	if !bucketExists {
		log.Print("Output bucket doesn't exist, creating...")
		err = minioClient.MakeBucket(context.Background(), outputBucketName, minio.MakeBucketOptions{})
		if err != nil {
			log.Fatal(err)
		}
	}

	return minioClient
}

func getObjectKeyFromUrl(u string) (string, error) {
	url, err := url.Parse(u)
	if err != nil {
		return "", err
	}

	objectKey := strings.TrimPrefix(url.Path, "/"+inputBucketName+"/")
	return objectKey, nil
}
