package main

import (
	"database/sql"
	"fmt"
	"log"

	pg "github.com/lib/pq"
)

func writeMetadataToDb(dbConn *sql.DB, filename string, objectUrl string) (int, error) {
	log.Printf("Writing metadata for object %s to db...", objectUrl)

	var dbFilename sql.NullString
	if filename == "" {
		dbFilename = sql.NullString{Valid: false}
	} else {
		dbFilename = sql.NullString{Valid: true, String: filename}
	}

	quotedTableName := pg.QuoteIdentifier(tableName)
	query := fmt.Sprintf(`INSERT INTO %v (url, file_name) VALUES ($1, $2) RETURNING id`, quotedTableName)

	var lastInsertId int
	err := dbConn.QueryRow(query, objectUrl, dbFilename).Scan(&lastInsertId)
	if err != nil {
		return 0, err
	}

	log.Printf("Inserted row with id %d", lastInsertId)
	return lastInsertId, nil
}

func initDb() *sql.DB {
	log.Print("Initializing db...")

	connectionString := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbName)

	dbConnection, err := sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Opened db connection")

	query := `SELECT EXISTS(
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
	);`

	var exists bool
	err = dbConnection.QueryRow(query, tableName).Scan(&exists)
	if err != nil {
		log.Fatal(err)
	}

	if !exists {
		log.Print("table doesn't exist, creating...")

		quotedTableName := pg.QuoteIdentifier(tableName)
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

		log.Print("Created table")
	}

	log.Print("Initialized db")

	return dbConnection
}
