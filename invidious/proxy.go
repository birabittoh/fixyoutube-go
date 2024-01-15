package invidious

import (
	"bytes"
	"io"
	"net/http"
)

func (c *Client) urlToBuffer(url string) (*bytes.Buffer, int64, int) {
	if url == "" {
		return nil, 0, http.StatusBadRequest
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Error(err) // bad request
		return nil, 0, http.StatusInternalServerError
	}

	resp, err := c.http.Do(req)
	if err != nil {
		logger.Error(err) // request failed
		return nil, 0, http.StatusGone
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, resp.StatusCode
	}

	if resp.ContentLength == 0 {
		return nil, 0, http.StatusNoContent
	}

	if resp.ContentLength > maxSizeBytes {
		logger.Debug("Content-Length exceeds max size.")
		return nil, 0, http.StatusBadRequest
	}
	defer resp.Body.Close()

	b := new(bytes.Buffer)
	l, err := io.Copy(b, resp.Body)
	if l != resp.ContentLength {
		logger.Debug("Content-Length is inconsistent.")
		return nil, 0, http.StatusBadRequest
	}

	return b, l, http.StatusOK
}

func (c *Client) findCompatibleFormat(video *Video) (*bytes.Buffer, int64, int) {
	for i := len(video.Formats) - 1; i >= 0; i-- {
		url := video.Formats[i].Url
		logger.Debug(url)
		b, l, httpStatus := c.urlToBuffer(url)
		if httpStatus == http.StatusOK {
			videoBuffer := NewVideoBuffer(b, l)
			c.buffers.Set(video.VideoId, videoBuffer)
			return b, l, i
		}
		logger.Debug("Format ", i, "failed with status code ", httpStatus)
	}
	return nil, 0, -1
}

func (c *Client) getBuffer(video Video) (*bytes.Buffer, int64, int) {
	vb, err := c.buffers.Get(video.VideoId)
	if err != nil {
		b, l, s := c.urlToBuffer(video.Url)
		if l > 0 {
			videoBuffer := NewVideoBuffer(b, l)
			c.buffers.Set(video.VideoId, videoBuffer)
		}
		return b, l, s
	}

	videoBuffer := NewVideoBuffer(vb.buffer, vb.length)
	return videoBuffer.buffer, videoBuffer.length, http.StatusOK
}

func (c *Client) ProxyVideoId(videoId string) (*bytes.Buffer, int64, int) {
	video, err := GetVideoDB(videoId)
	if err != nil {
		logger.Info("Cannot proxy a video that is not cached: https://youtu.be/", videoId)
		return nil, 0, http.StatusBadRequest
	}
	return c.getBuffer(*video)
}
