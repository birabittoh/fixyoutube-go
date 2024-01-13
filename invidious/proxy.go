package invidious

import (
	"bytes"
	"io"
	"net/http"
)

func (c *Client) ProxyVideo(url string) (*bytes.Buffer, int64, int) {
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
