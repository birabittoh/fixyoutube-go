package invidious

const createQueryVideos = `
CREATE TABLE IF NOT EXISTS videos (
    videoId TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    uploader TEXT NOT NULL,
    duration int NOT NULL,
    timestamp DATETIME DEFAULT (datetime('now')),
    expire DATETIME NOT NULL
);`

const createQueryFormats = `
CREATE TABLE IF NOT EXISTS formats (
    videoId TEXT,
	name TEXT,
    height TEXT NOT NULL,
    width TEXT NOT NULL,
    url TEXT,
	PRIMARY KEY (videoId, name),
	FOREIGN KEY(videoId) REFERENCES videos(videoId)
);`

const getVideoQuery = "SELECT * FROM videos WHERE videoId = (?);"
const getFormatQuery = "SELECT * FROM formats WHERE videoId = (?)"

const cacheVideoQuery = "INSERT OR REPLACE INTO videos (videoId, title, description, uploader, duration, expire) VALUES (?, ?, ?, ?, ?, ?);"
const cacheFormatQuery = "INSERT OR REPLACE INTO formats (videoId, name, height, width, url) VALUES (?, ?, ?, ?, ?);"

const clearQuery = "DELETE FROM videos;"
