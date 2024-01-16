package invidious

import (
	"bytes"
	"io"
	"net/http"
)

func (c *Client) urlToBuffer(url string) (*VideoBuffer, int) {
	if url == "" {
		return nil, http.StatusBadRequest
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Error(err) // bad request
		return nil, http.StatusInternalServerError
	}

	resp, err := c.http.Do(req)
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

	if resp.ContentLength > c.Options.MaxSizeBytes {
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

func (c *Client) findCompatibleFormat(video *Video) (*VideoBuffer, int) {
	for i := len(video.Formats) - 1; i >= 0; i-- {
		url := video.Formats[i].Url
		logger.Debug(url)
		vb, httpStatus := c.urlToBuffer(url)
		if httpStatus == http.StatusOK {
			videoBuffer := vb.Clone()
			c.buffers.Set(video.VideoId, videoBuffer)
			return vb, i
		}
		logger.Debug("Format ", i, "failed with status code ", httpStatus)
	}
	return nil, -1
}

func (c *Client) getBuffer(video Video) (*VideoBuffer, int) {
	vb, err := c.buffers.Get(video.VideoId)
	if err != nil {
		// no cache entry
		vb, s := c.urlToBuffer(video.Url)
		if vb != nil {
			if s == http.StatusOK && vb.Length > 0 {
				c.buffers.Set(video.VideoId, vb.Clone())
				return vb, s
			}
		}
		return nil, s
	}
	//cache entry
	videoBuffer := vb.Clone()
	return videoBuffer, http.StatusOK
}

func (c *Client) ProxyVideoId(videoId string) (*VideoBuffer, int) {
	video, err := GetVideoDB(videoId)
	if err != nil {
		logger.Info("Cannot proxy a video that is not cached: https://youtu.be/", videoId)
		return nil, http.StatusBadRequest
	}
	return c.getBuffer(*video)
}
