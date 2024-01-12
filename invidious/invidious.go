package invidious

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

const timeoutDuration = 10 * time.Minute
const maxSizeBytes = 20000000 // 20 MB
const instancesEndpoint = "https://api.invidious.io/instances.json?sort_by=api,type"
const videosEndpoint = "https://%s/api/v1/videos/%s?fields=videoId,title,description,author,lengthSeconds,size,formatStreams"

var expireRegex = regexp.MustCompile(`(?i)expire=(\d+)`)
var logger = logrus.New()

type Timeout struct {
	Instance  string
	Timestamp time.Time
}

type Client struct {
	http     *http.Client
	timeouts []Timeout
	Instance string
}

type Format struct {
	VideoId   string
	Name      string `json:"qualityLabel"`
	Height    int
	Width     int
	Url       string `json:"url"`
	Container string `json:"container"`
	Size      string `json:"size"`
}

type Video struct {
	VideoId     string   `json:"videoId"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Uploader    string   `json:"author"`
	Duration    int      `json:"lengthSeconds"`
	Formats     []Format `json:"formatStreams"`
	Timestamp   time.Time
	Expire      time.Time
	FormatIndex int
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

type HTTPError struct {
	StatusCode int
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("HTTP error: %d", e.StatusCode)
}

func (c *Client) fetchVideo(videoId string) (*Video, error) {
	if c.Instance == "" {
		err := c.NewInstance()
		if err != nil {
			logger.Fatal(err, "Could not get a new instance.")
		}
	}
	endpoint := fmt.Sprintf(videosEndpoint, c.Instance, url.QueryEscape(videoId))
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
		return nil, HTTPError{resp.StatusCode}
	}

	res := &Video{}
	err = json.Unmarshal(body, res)
	if err != nil {
		return nil, err
	}

	mp4Test := func(f Format) bool { return f.Container == "mp4" }
	res.Formats = filter(res.Formats, mp4Test)

	expireString := expireRegex.FindStringSubmatch(res.Formats[0].Url)
	expireTimestamp, err := strconv.ParseInt(expireString[1], 10, 64)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}
	res.Expire = time.Unix(expireTimestamp, 0)
	return res, nil
}

func (c *Client) GetVideo(videoId string, fromCache bool) (*Video, error) {
	logger.Info("Video https://youtu.be/", videoId, " was requested.")

	var video *Video
	var err error

	if fromCache {
		video, err = GetVideoDB(videoId)
		if err == nil {
			logger.Info("Found a valid cache entry.")
			return video, nil
		}
	}

	video, err = c.fetchVideo(videoId)

	if err != nil {
		if httpErr, ok := err.(HTTPError); ok {
			// handle HTTPError
			s := httpErr.StatusCode
			if s == http.StatusNotFound || s == http.StatusInternalServerError {
				logger.Debug("Video does not exist.")
				return nil, err
			}
			logger.Debug("Invidious HTTP error: ", httpErr.StatusCode)
		}
		// handle generic error
		logger.Error(err)
		err = c.NewInstance()
		if err != nil {
			logger.Error("Could not get a new instance: ", err)
			time.Sleep(10 * time.Second)
		}
		return c.GetVideo(videoId, true)
	}
	logger.Info("Retrieved by API.")

	err = CacheVideoDB(*video)
	if err != nil {
		logger.Warn("Could not cache video id: ", videoId)
		logger.Warn(err)
	}
	return video, nil
}

func (c *Client) isNotTimedOut(instance string) bool {
	for i := range c.timeouts {
		cur := c.timeouts[i]
		if instance == cur.Instance {
			return false
		}
	}
	return true
}

func (c *Client) NewInstance() error {
	now := time.Now()

	timeoutsTest := func(t Timeout) bool { return now.Sub(t.Timestamp) < timeoutDuration }
	c.timeouts = filter(c.timeouts, timeoutsTest)

	timeout := Timeout{c.Instance, now}
	c.timeouts = append(c.timeouts, timeout)

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
		return HTTPError{resp.StatusCode}
	}

	var jsonArray [][]interface{}
	err = json.Unmarshal(body, &jsonArray)
	if err != nil {
		logger.Error("Could not unmarshal JSON response for instances.")
		return err
	}

	for i := range jsonArray {
		instance := jsonArray[i][0].(string)
		instanceTest := func(t Timeout) bool { return t.Instance == instance }
		result := filter(c.timeouts, instanceTest)
		if len(result) == 0 {
			c.Instance = instance
			logger.Info("Using new instance: ", c.Instance)
			return nil
		}
	}
	logger.Error("Cannot find a valid instance.")
	return err
}

func (c *Client) ProxyVideo(url string, formatIndex int) (*bytes.Buffer, int64, int) {

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

	if resp.ContentLength > maxSizeBytes {
		logger.Debug("Format ", formatIndex, ": Content-Length exceeds max size.")
		return nil, 0, http.StatusBadRequest
	}
	defer resp.Body.Close()

	b := new(bytes.Buffer)
	l, err := io.Copy(b, resp.Body)
	if l != resp.ContentLength {
		return nil, 0, http.StatusBadRequest
	}

	return b, l, http.StatusOK
}

func NewClient(httpClient *http.Client) *Client {
	InitDB()
	return &Client{
		http:     httpClient,
		timeouts: []Timeout{},
		Instance: "",
	}
}
