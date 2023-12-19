package invidious

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var instancesEndpoint = "https://api.invidious.io/instances.json?sort_by=api,type"
var videosEndpoint = "https://%s/api/v1/videos/%s?fields=videoId,title,description,author,lengthSeconds,size,formatStreams"
var timeToLive, _ = time.ParseDuration("6h")
var maxSizeMB = 50

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
	Timestamp     time.Time
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

func (c *Client) fetchVideo(videoId string) (*Video, error) {
	if c.Instance == "" {
		err := c.NewInstance()
		if err != nil {
			log.Fatal(err, "Could not get a new instance.")
			os.Exit(1)
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

func (c *Client) GetVideo(videoId string) (*Video, error) {
	log.Println("Video", videoId, "was requested.")

	video, err := GetVideoDB(videoId)
	if err == nil {
		now := time.Now()
		delta := now.Sub(video.Timestamp)
		if delta < timeToLive {
			log.Println("Found a valid cache entry from", delta, "ago.")
			return video, nil
		}
	}

	video, err = c.fetchVideo(videoId)
	if err != nil {
		log.Println(err)
		err = c.NewInstance()
		if err != nil {
			log.Fatal("Could not get a new instance: ", err)
			time.Sleep(10)
		}
		return c.GetVideo(videoId)
	}
	log.Println("Retrieved by API.")

	CacheVideoDB(*video)
	return video, nil
}

func (c *Client) NewInstance() error {
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
		return fmt.Errorf(string(body))
	}

	var jsonArray [][]interface{}
	err = json.Unmarshal(body, &jsonArray)
	if err != nil {
		return err
	}

	c.Instance = jsonArray[0][0].(string)
	log.Println("Using new instance:", c.Instance)
	return nil
}

func (c *Client) ProxyVideo(videoId string, w http.ResponseWriter) error {
	video, err := GetVideoDB(videoId)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return err
	}

	req, err := http.NewRequest(http.MethodGet, video.Url, nil)
	if err != nil {
		log.Fatal(err)
		new_video, err := c.fetchVideo(videoId)
		if err != nil {
			log.Fatal("Url for", videoId, "expired:", err)
			return err
		}
		return c.ProxyVideo(new_video.VideoId, w)
	}

	req.Header.Add("Range", fmt.Sprintf("bytes=0-%d000000", maxSizeMB))
	resp, err := c.http.Do(req)
	if err != nil {
		log.Fatal(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	defer resp.Body.Close()

	w.Header().Set("content-type", "video/mp4")
	w.Header().Set("Status", "206") // Partial Content

	i, err := io.Copy(w, resp.Body)
	fmt.Println(i, err)
	return err
}

func NewClient(httpClient *http.Client) *Client {
	InitDB()
	return &Client{httpClient, ""}
}
