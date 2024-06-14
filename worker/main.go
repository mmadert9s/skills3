package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	workerCount int = 2 // Avoid OOMKilled when processing too many images at the same time

	inputBucketName  string = "uploaded"
	outputBucketName string = "processed"
	minioEndpoint    string = "minio:9000"
	accessKey        string = "miniouser"
	secretKey        string = "miniopassword"

	mqUser       string = "rabbitmquser"
	mqPassword   string = "rabbitmqpassword"
	mqHost       string = "rabbitmq"
	exchangeName string = "events"
	queueName    string = "uploaded"
)

type UploadMessage struct {
	Id        int    `json:"id"`
	ObjectUrl string `json:"url"`
}

func main() {
	minioClient := initMinio()

	mqConnection, mqChannel := initMQ()
	defer mqChannel.Close()
	defer mqConnection.Close()

	messages := initConsumer(mqChannel)

	jobs := make(chan amqp.Delivery)

	for w := 1; w <= workerCount; w++ {
		go worker(w, jobs, minioClient)
	}

	log.Printf("Waiting for messages...")
	for d := range messages {
		jobs <- d
	}
}

func worker(id int, jobs <-chan amqp.Delivery, mc *minio.Client) {
	log.Printf("Started worker %d, listening for jobs...", id)
	for j := range jobs {
		processMessage(j, mc)
	}
}

func processMessage(d amqp.Delivery, minioClient *minio.Client) {
	log.Printf("Processing message: %s", d.Body)

	var message UploadMessage
	err := json.Unmarshal(d.Body, &message)
	if err != nil {
		log.Printf("unexpected error decoding json %s : %v", d.Body, err)
		rejectMessage(d)
		return
	}

	objectKey, err := getObjectKeyFromUrl(message.ObjectUrl) // it would be easier to simply pass the object key in the message
	if err != nil {
		log.Printf("unexpected error getting object key from %s : %v", objectKey, err)
		rejectMessage(d)
		return
	}

	// Create a unique directory to store the image and the output, to avoid goroutines overwriting each
	// other in the case of object keys being equal
	basePath := path.Join("tmp", uuid.NewString())
	err = os.MkdirAll(path.Join(basePath, "output"), os.ModePerm)
	if err != nil {
		log.Printf("unexpected error creating path %s : %v", basePath, err)
		rejectMessage(d)
		return
	}

	inPath := path.Join(basePath, objectKey)
	localFile, err := os.Create(inPath)
	if err != nil {
		log.Printf("unexpected error creating file %s : %v", objectKey, err)
		rejectMessage(d)
		return
	}

	log.Printf("Getting objectkey %s from minio...", objectKey)
	object, err := minioClient.GetObject(context.Background(), inputBucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("unexpected error getting file %s from minio : %v", objectKey, err)
		rejectMessage(d)
		return
	}

	_, err = io.Copy(localFile, object)
	if err != nil {
		log.Printf("unexpected error while downloading file %s : %v", basePath, err)
		rejectMessage(d)
		return
	}

	object.Close()
	localFile.Close()

	outPath := path.Join(basePath, "output", objectKey)

	log.Print("Executing face recognition script...")
	cmd := exec.Command(
		"python3", "/workdir/yolo_opencv.py",
		"--image", inPath,
		"--output", outPath,
		"--config", "/workdir/yolov3.cfg",
		"--weights", "/workdir/yolov3.weights",
		"--classes", "/workdir/yolov3.txt",
	)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	if err != nil {
		log.Print("python execution error:", err)
		log.Print("stderr:", stderrBuf.String())
		rejectMessage(d)
		return
	} else {
		log.Print("Python execution ok")
	}

	outFile, err := os.Open(outPath)
	if err != nil {
		log.Printf("unexpected error opening output file: %v", err)
		rejectMessage(d)
		return
	}

	outFileInfo, err := outFile.Stat()
	if err != nil {
		log.Printf("unexpected error getting output file info: %v", err)
		rejectMessage(d)
		return
	}

	info, err := minioClient.PutObject(context.Background(), outputBucketName, objectKey, outFile, outFileInfo.Size(), minio.PutObjectOptions{})
	if err != nil {
		log.Printf("unexpected error putting output file: %v", err)
		rejectMessage(d)
		return
	}
	log.Printf("Uploaded object %v", info.Key)

	outFile.Close()

	err = minioClient.RemoveObject(context.Background(), inputBucketName, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		log.Printf("unexpected error deleting original image %s : %v", objectKey, err)
		rejectMessage(d)
		return
	}

	d.Ack(false)

	// cleanup
	os.RemoveAll(basePath)

	log.Printf("Done processing %s", objectKey)
}

func rejectMessage(d amqp.Delivery) {
	log.Printf("Rejecting message %s", d.Body)
	d.Nack(false, false)
}
