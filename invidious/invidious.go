package invidious

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Client struct {
	http     *http.Client
	Instance string
}

type Format struct {
	Url       string `json:"url"`
	Container string `json:"container"`
	Size      string `json:"size"`
}

type Video struct {
	VideoId       string   `json:"videoId"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Uploader      string   `json:"author"`
	Duration      int      `json:"lengthSeconds"`
	FormatStreams []Format `json:"formatStreams"`
	Url           string
	Height        int
	Width         int
}

func filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func parseOrZero(number string) int {
	res, err := strconv.Atoi(number)
	if err != nil {
		return 0
	}
	return res
}

func (c *Client) FetchEverything(videoId string) (*Video, error) {
	endpoint := fmt.Sprintf("https://%s/api/v1/videos/%s?fields=videoId,title,description,author,lengthSeconds,size,formatStreams", c.Instance, url.QueryEscape(videoId))
	resp, err := c.http.Get(endpoint)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(body))
	}

	res := &Video{}
	err = json.Unmarshal(body, res)
	if err != nil {
		return nil, err
	}

	mp4Test := func(f Format) bool { return f.Container == "mp4" }
	mp4Formats := filter(res.FormatStreams, mp4Test)
	myFormat := mp4Formats[len(mp4Formats)-1]
	mySize := strings.Split(myFormat.Size, "x")

	res.Url = myFormat.Url
	res.Width = parseOrZero(mySize[0])
	res.Height = parseOrZero(mySize[1])

	return res, err
}

func NewClient(httpClient *http.Client, instance string) *Client {
	return &Client{httpClient, instance}
}
