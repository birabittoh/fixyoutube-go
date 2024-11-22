package invidious

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Format struct {
	Name      string `json:"qualityLabel"`
	Url       string `json:"url"`
	Container string `json:"container"`
	Size      string `json:"size"`
	Itag      string `json:"itag"`
}

type VideoThumbnail struct {
	Quality string `json:"quality"`
	URL     string `json:"url"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

func (c *Client) fetchVideo(videoId string) (*Video, int) {
	endpoint := fmt.Sprintf(videosEndpoint, c.Instance, url.QueryEscape(videoId))
	resp, err := c.http.Get(endpoint)
	if err != nil {
		logger.Error(err)
		return nil, http.StatusInternalServerError
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, http.StatusNotFound
	}

	if resp.StatusCode != http.StatusOK {
		logger.Warn("Invidious gave the following status code: ", resp.StatusCode)
		return nil, resp.StatusCode
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error(err)
		return nil, http.StatusInternalServerError
	}

	res := &Video{}
	err = json.Unmarshal(body, res)
	if err != nil {
		logger.Error(err)
		return nil, http.StatusInternalServerError
	}

	if len(res.VideoThumbnails) > 0 {
		res.Thumbnail = res.VideoThumbnails[0].URL
	}

	mp4Test := func(f Format) bool { return f.Itag == "18" }
	res.Formats = filter(res.Formats, mp4Test)

	expireString := expireRegex.FindStringSubmatch(res.Formats[0].Url)
	expireTimestamp, err := strconv.ParseInt(expireString[1], 10, 64)
	if err != nil {
		logger.Error(err)
		return nil, http.StatusInternalServerError
	}
	res.Expire = time.Unix(expireTimestamp, 0)

	vb, i := c.findCompatibleFormat(res)
	if i < 0 {
		logger.Warn("No compatible formats found for video.")
		res.Url = ""
	} else {
		videoBuffer := vb.Clone()
		c.buffers.Set(videoId, videoBuffer)
		res.Url = res.Formats[i].Url
	}

	return res, http.StatusOK
}

func (c *Client) NewInstance() error {
	if c.Instance != "" {
		err := fmt.Errorf("generic error")
		c.timeouts.Set(c.Instance, &err)
	}

	resp, err := c.http.Get(instancesEndpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	var jsonArray [][]interface{}
	err = json.Unmarshal(body, &jsonArray)
	if err != nil {
		logger.Error("Could not unmarshal JSON response for instances.")
		return err
	}

	for i := range jsonArray {
		instance := jsonArray[i][0].(string)
		if !c.timeouts.Has(instance) {
			c.Instance = instance
			logger.Info("Using new instance: ", c.Instance)
			return nil
		}
	}

	return fmt.Errorf("cannot find a valid instance")
}
