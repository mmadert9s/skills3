package main

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/minio/minio-go"

	"database/sql"

	pg "github.com/lib/pq"
)

var minioClient *minio.Client
var bucketName string = "uploaded"
var minioEndpoint string = "minio:9000"
var accessKey string = "miniouser"
var secretKey string = "miniopassword"
var minioUseSSL bool = false

var dbConnection *sql.DB
var tableName string = "uploads"

func main() {
	log.Print("main")

	initMinio()
	initDb()
	defer dbConnection.Close()

	http.HandleFunc("POST /upload", fileUpload)
	http.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	http.ListenAndServe(":8080", nil)
}

func fileUpload(w http.ResponseWriter, req *http.Request) {
	log.Print("fileUpload")

	if req.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	uuid := uuid.NewString()
	objectName := fmt.Sprintf("uploaded-%s", uuid)
	n, err := minioClient.PutObject(bucketName, objectName, req.Body, req.ContentLength, minio.PutObjectOptions{})
	if err != nil {
		log.Printf("unexpected error while putting object: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Printf("wrote %d bytes uploading object %v", n, objectName)

	objectUrl := url.URL{
		Scheme: "http",
		Host:   minioEndpoint,
		Path:   "/" + bucketName + "/" + objectName,
	}

	filename, err := getFilenameFromRequest(req)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var dbFilename sql.NullString
	if filename == "" {
		dbFilename = sql.NullString{Valid: false}
	} else {
		dbFilename = sql.NullString{Valid: true, String: filename}
	}

	quotedTableName := pg.QuoteIdentifier(tableName)
	query := fmt.Sprintf(`INSERT INTO %v (url, file_name) VALUES ($1, $2)`, quotedTableName)

	_, err = dbConnection.Exec(query, objectUrl.String(), dbFilename)
	if err != nil {
		log.Printf("unexpected error while inserting new object into db: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Printf("inserted row")
}

// getFilenameFromRequest retrieves the filename parameter from the Content-Disposition header. If none is set, returns an empty string.
func getFilenameFromRequest(r *http.Request) (string, error) {
	contentDisp := r.Header.Get("Content-Disposition")
	if contentDisp == "" {
		fmt.Println("no content dispo")
		return "", nil
	}

	_, params, err := mime.ParseMediaType(contentDisp)
	if err != nil {
		return "", fmt.Errorf("malformed Content-Disposition header")
	}

	return params["filename"], nil
}

// initMinio creates a client and creates the bucket to uploads images to if it doesn't already exist.
func initMinio() {
	log.Print("init minio")

	var err error
	minioClient, err = minio.New(minioEndpoint, accessKey, secretKey, minioUseSSL)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("created minio client!")

	bucketExists, err := minioClient.BucketExists(bucketName)
	if err != nil {
		log.Fatal(err)
	}

	if !bucketExists {
		log.Print("Bucket doesn't exist, creating...")
		if err = minioClient.MakeBucket(bucketName, ""); err != nil {
			log.Fatal(err)
		}
	}

	log.Print("minio initialized")
}

func initDb() {
	log.Print("init db")

	connectionString := "postgres://postgresuser:postgrespassword@postgres/postgresuser?sslmode=disable"
	var err error
	dbConnection, err = sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("opened db connection!")

	quotedTableName := pg.QuoteIdentifier(tableName)
	query := `SELECT EXISTS(
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
	);`

	var exists bool
	err = dbConnection.QueryRow(query, quotedTableName).Scan(&exists)
	if err != nil {
		log.Fatal(err)
	}

	if !exists {
		log.Print("table doesn't exist, creating...")

		query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %v (
			id SERIAL PRIMARY KEY,
			file_name TEXT,
			url TEXT,
			url_processed TEXT,
			uploaded TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			processed TIMESTAMP
		);`, quotedTableName)

		_, err = dbConnection.Exec(query)

		if err != nil {
			log.Fatal(err)
		}

		log.Print("created table")
	}

	log.Print("initialized db!")
}
