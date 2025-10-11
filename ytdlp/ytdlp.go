package ytdlp

import (
	"bytes"
	"encoding/json"
	"iter"
	"os/exec"
	"time"

	"github.com/birabittoh/myks"
	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger
var buffers *myks.KeyStore[VideoBuffer]
var cache *myks.KeyStore[Video]

type VideoBuffer struct {
	Buffer *bytes.Buffer
	Length int64
}

type Video struct {
	VideoID       string                `json:"id"`
	Title         string                `json:"title"`
	Description   string                `json:"description"`
	Author        string                `json:"uploader"`
	AuthorURL     string                `json:"uploader_url"`
	LengthSeconds int                   `json:"duration"`
	Thumbnails    []Thumbnail           `json:"thumbnails"`
	Formats       []Format              `json:"formats"`
	Subtitles     map[string][]Subtitle `json:"subtitles"`
}

type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Format struct {
	Itag          string  `json:"format_id"`
	URL           string  `json:"url"`
	Type          string  `json:"format_note"`
	Resolution    string  `json:"resolution"`
	FPS           float64 `json:"fps"`
	AudioChannels int     `json:"audio_channels"`
	Bitrate       float64 `json:"tbr"`
	Ext           string  `json:"ext"`
	VCodec        string  `json:"vcodec"`
	ACodec        string  `json:"acodec"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
}

type Subtitle struct {
	URL  string `json:"url"`
	Ext  string `json:"ext"`
	Name string `json:"name"`
}

func Init(logLevel logrus.Level) {
	logger = logrus.New()
	logger.SetLevel(logLevel)

	buffers = myks.New[VideoBuffer](time.Minute)
	cache = myks.New[Video](10 * time.Minute)
}

func GetVideo(videoID string) (*Video, error) {
	video, err := cache.Get(videoID)
	if err == nil {
		return video, nil
	}

	return getVideoNoCache(videoID)
}

func GetVideoNoCache(videoID string) (*Video, error) {
	return getVideoNoCache(videoID)
}

func getVideoNoCache(videoID string) (*Video, error) {
	cmd := exec.Command("yt-dlp", "-j", "--no-warnings", "https://www.youtube.com/watch?v="+videoID)
	output, err := cmd.Output()
	if err != nil {
		logger.Error("yt-dlp error: ", err)
		return nil, err
	}

	var video Video
	err = json.Unmarshal(output, &video)
	if err != nil {
		logger.Error("JSON unmarshal error: ", err)
		return nil, err
	}

	cache.Set(videoID, video, 10*time.Minute)
	return &video, nil
}

func GetCaptions(videoID string, language string) ([]byte, error) {
	cmd := exec.Command("yt-dlp", "--skip-download", "--write-subs", "--sub-lang", language, "--sub-format", "vtt", "--print", "filename", "-o", "-", "https://www.youtube.com/watch?v="+videoID)
	output, err := cmd.Output()
	if err != nil {
		logger.Error("yt-dlp captions error: ", err)
		return nil, err
	}
	return output, nil
}

func GetCachedVideos() iter.Seq[string] {
	return cache.Keys()
}

func (video *Video) GetFormatStreams() []Format {
	var streams []Format
	for _, f := range video.Formats {
		if f.Itag == "22" || f.Itag == "18" { // 720p or 360p mp4
			streams = append(streams, f)
		}
	}
	return streams
}

func (video *Video) GetAdaptiveFormats() []Format {
	var adaptive []Format
	for _, f := range video.Formats {
		if f.VCodec == "none" || f.ACodec == "none" {
			adaptive = append(adaptive, f)
		}
	}
	return adaptive
}

func (video *Video) GetVideoThumbnails() []Thumbnail {
	return video.Thumbnails
}

func (video *Video) GetCaptions() []Caption {
	var captions []Caption
	for lang, subs := range video.Subtitles {
		if len(subs) > 0 {
			captions = append(captions, Caption{
				Label:        lang,
				LanguageCode: lang,
			})
		}
	}
	return captions
}

type Caption struct {
	Label        string
	LanguageCode string
}

func GetVideoURL(video Video) string {
	streams := video.GetFormatStreams()

	if len(streams) == 0 {
		return ""
	}
	return streams[0].URL
}

func NewVideoBuffer(b *bytes.Buffer, l int64) *VideoBuffer {
	d := new(bytes.Buffer)
	d.Write(b.Bytes())
	return &VideoBuffer{Buffer: d, Length: l}
}

func (vb *VideoBuffer) Clone() *VideoBuffer {
	return NewVideoBuffer(vb.Buffer, vb.Length)
}

func (vb *VideoBuffer) ValidateLength() bool {
	return vb.Length > 0 && vb.Length == int64(vb.Buffer.Len())
}
