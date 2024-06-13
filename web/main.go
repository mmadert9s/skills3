package main

import (
	"fmt"
	"log"
	"mime"
	"net/http"

	"github.com/minio/minio-go/v7"

	amqp "github.com/rabbitmq/amqp091-go"

	"database/sql"
)

const (
	bucketName    string = "uploaded"
	minioEndpoint string = "minio:9000"
	accessKey     string = "miniouser"
	secretKey     string = "miniopassword"

	dbUser     string = "postgresuser"
	dbPassword string = "postgrespassword"
	dbHost     string = "postgres"
	dbName     string = "postgresuser"
	tableName  string = "uploads"

	mqUser       string = "rabbitmquser"
	mqPassword   string = "rabbitmqpassword"
	mqHost       string = "rabbitmq"
	exchangeName string = "events"
	queueName    string = "uploaded"
)

func main() {
	log.Print("Main")

	var minioClient *minio.Client
	var dbConnection *sql.DB
	var mqConnection *amqp.Connection
	var mqChannel *amqp.Channel

	minioClient = initMinio()

	dbConnection = initDb()
	defer dbConnection.Close()

	mqConnection, mqChannel = initMQ()
	defer mqChannel.Close()
	defer mqConnection.Close()

	http.HandleFunc("POST /upload", func(w http.ResponseWriter, r *http.Request) {
		fileUpload(w, r, minioClient, dbConnection, mqChannel)
	})

	http.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.ListenAndServe(":8080", nil)
}

func fileUpload(w http.ResponseWriter, req *http.Request, mc *minio.Client, dbConn *sql.DB, mqChannel *amqp.Channel) {
	log.Print("fileUpload")

	if req.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	filename, err := getFilenameFromRequest(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	objectUrl, err := uploadToS3(mc, filename, req.Body, req.ContentLength)
	if err != nil {
		log.Printf("unexpected error while uploading object to s3: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	lastInsertId, err := writeMetadataToDb(dbConn, filename, objectUrl)
	if err != nil {
		log.Printf("unexpected error while inserting object metadata into db: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	err = publishObjectUploadedMessage(mqChannel, objectUrl, lastInsertId)
	if err != nil {
		log.Printf("unexpected error while publishing to mq: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Print("Done handling file")
}

// getFilenameFromRequest retrieves the filename parameter from the Content-Disposition header. If none is set, returns an empty string.
func getFilenameFromRequest(r *http.Request) (string, error) {
	contentDisp := r.Header.Get("Content-Disposition")
	if contentDisp == "" {
		return "", nil
	}

	_, params, err := mime.ParseMediaType(contentDisp)
	if err != nil {
		return "", fmt.Errorf("malformed Content-Disposition header")
	}

	return params["filename"], nil
}
