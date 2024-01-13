package invidious

import (
	"fmt"
	"net/http"
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

	video, httpErr := c.fetchVideo(videoId)

	switch httpErr {
	case http.StatusOK:
		logger.Info("Retrieved by API.")
		break
	case http.StatusNotFound:
		logger.Debug("Video does not exist or can't be retrieved.")
		return nil, err
	default:
		fallthrough
	case http.StatusInternalServerError:
		err = c.NewInstance()
		if err != nil {
			logger.Error("Could not get a new instance: ", err)
			time.Sleep(10 * time.Second)
		}
		return c.GetVideo(videoId, true)
	}

	err = CacheVideoDB(*video)
	if err != nil {
		logger.Warn("Could not cache video id: ", videoId)
		logger.Warn(err)
	}
	return video, nil
}

func NewClient(httpClient *http.Client) *Client {
	InitDB()
	client := &Client{
		http:     httpClient,
		timeouts: []Timeout{},
		Instance: "",
	}
	err := client.NewInstance()
	if err != nil {
		logger.Fatal(err)
	}
	return client
}
