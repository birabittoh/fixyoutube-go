package invidious

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/birabittoh/rabbitpipe"
)

const MaxSizeBytes = int64(20 * 1024 * 1024)

func urlToBuffer(url string) (*VideoBuffer, int) {
	if url == "" {
		return nil, http.StatusBadRequest
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Error(err) // bad request
		return nil, http.StatusInternalServerError
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error(err) // request failed
		return nil, http.StatusGone
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode
	}

	if resp.ContentLength == 0 {
		return nil, http.StatusNoContent
	}

	if resp.ContentLength > MaxSizeBytes {
		logger.Debug("Content-Length exceeds max size.")
		return nil, http.StatusBadRequest
	}
	defer resp.Body.Close()

	b := new(bytes.Buffer)
	l, _ := io.Copy(b, resp.Body)
	if l != resp.ContentLength {
		logger.Debug("Content-Length is inconsistent.")
		return nil, http.StatusBadRequest
	}

	return &VideoBuffer{b, l}, http.StatusOK
}

func getBuffer(video rabbitpipe.Video) (*VideoBuffer, int) {
	vb, err := buffers.Get(video.VideoID)
	if err != nil {
		// cached buffer not found
		vb, s := urlToBuffer(GetVideoURL(video))
		if vb != nil {
			if s == http.StatusOK && vb.Length > 0 {
				buffers.Set(video.VideoID, *vb.Clone(), 5*time.Minute)
				return vb, s
			}
		}
		return nil, s
	}

	// cached buffer found
	return vb.Clone(), http.StatusOK
}

func ProxyVideoId(videoID string) (*VideoBuffer, int) {
	video, err := RP.GetVideo(videoID)
	if err != nil {
		logger.Info("Cannot get video: https://youtu.be/", videoID)
		return nil, http.StatusBadRequest
	}
	return getBuffer(*video)
}
