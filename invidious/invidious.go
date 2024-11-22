package invidious

import (
	"bytes"
	"net/http"
	"regexp"
	"time"

	"github.com/BiRabittoh/fixyoutube-go/volatile"
	"github.com/sirupsen/logrus"
)

const instancesEndpoint = "https://api.invidious.io/instances.json?sort_by=api,type"
const videosEndpoint = "https://%s/api/v1/videos/%s?fields=videoId,title,description,author,lengthSeconds,size,formatStreams"

var expireRegex = regexp.MustCompile(`(?i)expire=(\d+)`)
var logger = logrus.New()

type ClientOptions struct {
	CacheDuration   time.Duration
	TimeoutDuration time.Duration
	CleanupInterval time.Duration
	MaxSizeBytes    int64
}

type VideoBuffer struct {
	Buffer *bytes.Buffer
	Length int64
}

type Client struct {
	http     *http.Client
	timeouts *volatile.Volatile[string, error]
	buffers  *volatile.Volatile[string, VideoBuffer]
	Instance string
	Options  ClientOptions
}

type Video struct {
	VideoId         string           `json:"videoId"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	Uploader        string           `json:"author"`
	VideoThumbnails []VideoThumbnail `json:"videoThumbnails"`
	Duration        int              `json:"lengthSeconds"`
	Formats         []Format         `json:"formatStreams"`
	Expire          time.Time
	Url             string
	Thumbnail       string
}

func filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func NewVideoBuffer(b *bytes.Buffer, l int64) *VideoBuffer {
	d := new(bytes.Buffer)
	d.Write(b.Bytes())

	return &VideoBuffer{
		Buffer: d,
		Length: l,
	}
}

func (vb *VideoBuffer) Clone() *VideoBuffer {
	return NewVideoBuffer(vb.Buffer, vb.Length)
}

func (vb *VideoBuffer) ValidateLength() bool {
	return vb.Length > 0 && vb.Length == int64(vb.Buffer.Len())
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
	case http.StatusNotFound:
		logger.Debug("Video does not exist or can't be retrieved.")
		return nil, err
	default:
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

func NewClient(httpClient *http.Client, options ClientOptions) *Client {
	InitDB()
	timeouts := volatile.NewVolatile[string, error](options.TimeoutDuration, options.CleanupInterval)
	buffers := volatile.NewVolatile[string, VideoBuffer](options.CacheDuration, options.CleanupInterval)
	client := &Client{
		http:     httpClient,
		timeouts: timeouts,
		buffers:  buffers,
		Options:  options,
	}
	err := client.NewInstance()
	if err != nil {
		logger.Fatal(err)
	}
	return client
}
