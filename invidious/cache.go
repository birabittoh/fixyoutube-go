package invidious

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var dbConnectionString = "file:cache.sqlite?cache=shared&mode="
var createQuery = `
CREATE TABLE IF NOT EXISTS videos (
    videoId TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    uploader TEXT NOT NULL,
    duration int NOT NULL,
    height TEXT NOT NULL,
    width TEXT NOT NULL,
    url TEXT,
    timestamp DATETIME DEFAULT (datetime('now'))
);`
var getVideoQuery = "SELECT * FROM videos WHERE videoId = (?);"
var cacheVideoQuery = "INSERT OR REPLACE INTO videos (videoId, title, description, uploader, duration, height, width, url) VALUES (?, ?, ?, ?, ?, ?, ?, ?);"
var clearQuery = "DELETE FROM videos;"

func getDb(mode string) *sql.DB {
	db, err := sql.Open("sqlite3", dbConnectionString+mode)
	if err != nil {
		log.Fatal("Error opening database")
	}
	db.SetMaxOpenConns(1)
	return db
}

func InitDB() {
	db := getDb("rwc")
	defer db.Close()

	_, err := db.Exec(createQuery)
	if err != nil {
		log.Printf("%q: %s\n", err, createQuery)
		return
	}
}

func CacheVideoDB(v Video) {
	db := getDb("rw")
	defer db.Close()

	stmt, err := db.Prepare(cacheVideoQuery)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(v.VideoId, v.Title, v.Description, v.Uploader, v.Duration, v.Height, v.Width, v.Url)
	if err != nil {
		log.Printf("%q: %s\n", err, cacheVideoQuery)
		return
	}
}

func GetVideoDB(videoId string) (*Video, error) {
	db := getDb("ro")
	defer db.Close()

	stmt, err := db.Prepare(getVideoQuery)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	t := &Video{}
	err = stmt.QueryRow(videoId).Scan(&t.VideoId, &t.Title, &t.Description, &t.Uploader, &t.Duration, &t.Height, &t.Width, &t.Url, &t.Timestamp)
	if err != nil {
		//log.Printf("%q: %s\n", err, getVideoQuery)
		return &Video{}, err
	}
	return t, nil
}

func ClearDB() {
	db := getDb("rw")
	defer db.Close()

	stmt, err := db.Prepare(clearQuery)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec()
	if err != nil {
		log.Printf("%q: %s\n", err, cacheVideoQuery)
		return
	}
}
