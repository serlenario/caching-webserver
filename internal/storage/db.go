package storage

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
)

var db *sql.DB

func InitDB(databaseURL string) error {
	var err error
	db, err = sql.Open("postgres", databaseURL)
	if err != nil {
		return err
	}

	err = db.Ping()
	if err != nil {
		return err
	}

	return createTables()
}

func createTables() error {
	userTable := `
    CREATE TABLE IF NOT EXISTS users (
        id SERIAL PRIMARY KEY,
        login VARCHAR(255) UNIQUE NOT NULL,
        password VARCHAR(255) NOT NULL
    );`

	documentTable := `
    CREATE TABLE IF NOT EXISTS documents (
        id VARCHAR(255) PRIMARY KEY,
        owner_id INTEGER REFERENCES users(id),
        name VARCHAR(255),
        mime VARCHAR(255),
        file BOOLEAN,
        public BOOLEAN,
        created_at TIMESTAMP,
        data BYTEA,
        access_grant TEXT[]
    );`

	_, err := db.Exec(userTable)
	if err != nil {
		return err
	}

	_, err = db.Exec(documentTable)
	return err
}

func CloseDB() {
	if err := db.Close(); err != nil {
		log.Printf("Error while closing the database: %v", err)
	}
}

func GetDB() *sql.DB {
	return db
}
