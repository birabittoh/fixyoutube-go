package invidious

const createQueryVideos = `
CREATE TABLE IF NOT EXISTS videos (
    videoId TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    uploader TEXT NOT NULL,
    duration int NOT NULL,
    url TEXT NOT NULL,
    expire DATETIME NOT NULL
);`

const getVideoQuery = "SELECT * FROM videos WHERE videoId = (?);"

const cacheVideoQuery = "INSERT OR REPLACE INTO videos (videoId, title, description, uploader, duration, url, expire) VALUES (?, ?, ?, ?, ?, ?, ?);"

const clearQuery = "DELETE FROM videos;"
