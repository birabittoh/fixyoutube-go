package invidious

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

const dbConnectionString = "file:cache.sqlite?cache=shared&mode="

func getDb(mode string) *sql.DB {
	db, err := sql.Open("sqlite3", dbConnectionString+mode)
	if err != nil {
		logger.Error("Could not open DB:", err)
		return nil
	}
	db.SetMaxOpenConns(1)
	return db
}

func InitDB() {
	db := getDb("rwc")
	defer db.Close()

	_, err := db.Exec(createQueryVideos)
	if err != nil {
		logger.Errorf("%q: %s\n", err, createQueryVideos)
		return
	}
}

func CacheVideoDB(v Video) error {
	db := getDb("rw")
	defer db.Close()

	cacheVideo, err := db.Prepare(cacheVideoQuery)
	if err != nil {
		logger.Error("Could not cache video: ", err)
		return err
	}
	defer cacheVideo.Close()

	_, err = cacheVideo.Exec(v.VideoId, v.Title, v.Description, v.Uploader, v.Duration, v.Url, v.Expire)
	if err != nil {
		logger.Error("Could not cache video: ", err)
		return err
	}

	return nil
}

func GetVideoDB(videoId string) (*Video, error) {
	db := getDb("ro")
	defer db.Close()

	getVideo, err := db.Prepare(getVideoQuery)
	if err != nil {
		logger.Error("Could not get video: ", err)
		return nil, err
	}
	defer getVideo.Close()

	v := &Video{}
	err = getVideo.QueryRow(videoId).Scan(&v.VideoId, &v.Title, &v.Description, &v.Uploader, &v.Duration, &v.Url, &v.Timestamp, &v.Expire)
	if err != nil {
		logger.Debug("Could not get video:", err)
		return nil, err
	}

	if v.Timestamp.After(v.Expire) {
		logger.Info("Video has expired.")
		return nil, fmt.Errorf("expired")
	}

	return v, nil
}

func ClearDB() {
	db := getDb("rw")
	defer db.Close()

	stmt, err := db.Prepare(clearQuery)
	if err != nil {
		logger.Error("Could not clear DB:", err)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec()
	if err != nil {
		logger.Error("Could not clear DB:", err)
		return
	}
}
