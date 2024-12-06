package main

import (
	"embed"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/birabittoh/fixyoutube-go/invidious"
	"github.com/birabittoh/rabbitpipe"
)

const templatesDirectory = "templates/"

var (
	//go:embed templates/index.html templates/video.html
	templates     embed.FS
	indexTemplate = template.Must(template.ParseFS(templates, templatesDirectory+"index.html"))
	videoTemplate = template.Must(template.New("video.html").Funcs(template.FuncMap{"parseFormat": parseFormat}).ParseFS(templates, templatesDirectory+"video.html"))
	// userAgentRegex = regexp.MustCompile(`(?i)bot|facebook|embed|got|firefox\/92|firefox\/38|curl|wget|go-http|yahoo|generator|whatsapp|preview|link|proxy|vkshare|images|analyzer|index|crawl|spider|python|cfnetwork|node`)
	videoRegex = regexp.MustCompile(`(?i)^[a-z0-9_-]{11}$`)
)

func parseFormat(f rabbitpipe.Format) (res string) {
	isAudio := f.AudioChannels > 0

	if isAudio {
		bitrate, err := strconv.Atoi(f.Bitrate)
		if err != nil {
			logger.Error("Failed to convert bitrate to integer.")
			return
		}
		res = strconv.Itoa(bitrate/1000) + "kbps"
	} else {
		res = f.Resolution
	}

	mime := strings.Split(f.Type, ";")
	res += " - " + mime[0]

	codecs := " (" + strings.Split(mime[1], "\"")[1] + ")"

	if !isAudio {
		res += fmt.Sprintf(" (%d FPS)", f.FPS)
	}

	res += codecs
	return
}

func getItag(formats []rabbitpipe.Format, itag string) *rabbitpipe.Format {
	for _, f := range formats {
		if f.Itag == itag {
			return &f
		}
	}

	return nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	err := indexTemplate.Execute(w, nil)
	if err != nil {
		logger.Error("Failed to fill index template.")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func videoHandler(videoID string, w http.ResponseWriter, r *http.Request) {
	url := "https://www.youtube.com/watch?v=" + videoID

	if !videoRegex.MatchString(videoID) {
		logger.Info("Invalid video ID: ", videoID)
		http.Error(w, "Invalid video ID.", http.StatusBadRequest)
		return
	}

	video, err := invidious.RP.GetVideo(videoID)
	if err != nil || video == nil {
		logger.Info("Wrong video ID: ", videoID)
		http.Error(w, "Wrong video ID.", http.StatusNotFound)
		return
	}

	if invidious.GetVideoURL(*video) == "" {
		logger.Debug("No URL available. Redirecting.")
		http.Redirect(w, r, url, http.StatusFound)
		return
	}

	err = videoTemplate.Execute(w, video)
	if err != nil {
		logger.Error("Failed to fill video template.")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func watchHandler(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.String())
	if err != nil {
		logger.Error("Failed to parse URL: ", r.URL.String())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	q := u.Query()
	videoID := q.Get("v")
	videoHandler(videoID, w, r)
}

func shortHandler(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("videoID")
	videoHandler(videoID, w, r)
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("videoID")

	vb, s := invidious.ProxyVideoId(videoID)
	if s != http.StatusOK {
		logger.Error("proxyHandler() failed. Final code: ", s)
		http.Error(w, http.StatusText(s), s)
		return
	}
	if !vb.ValidateLength() {
		logger.Error("Buffer length is inconsistent.")
		status := http.StatusInternalServerError
		http.Error(w, http.StatusText(status), status)
		return
	}
	h := w.Header()
	h.Set("Status", "200")
	h.Set("Content-Type", "video/mp4")
	h.Set("Content-Length", strconv.FormatInt(vb.Length, 10))
	io.Copy(w, vb.Buffer)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	videoID := r.FormValue("video")
	if videoID == "" {
		http.Error(w, "Missing video ID", http.StatusBadRequest)
		return
	}

	if !videoRegex.MatchString(videoID) {
		logger.Println("Invalid video ID:", videoID)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	itag := r.FormValue("itag")
	if itag == "" {
		http.Error(w, "not found", http.StatusBadRequest)
		return
	}

	video, err := invidious.RP.GetVideo(videoID)
	if err != nil || video == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	format := getItag(video.FormatStreams, itag)
	if format == nil {
		format = getItag(video.AdaptiveFormats, itag)
		if format == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}

	http.Redirect(w, r, format.URL, http.StatusFound)
}

func refreshHandler(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("videoID")
	if videoID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !videoRegex.MatchString(videoID) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	video, err := invidious.RP.GetVideoNoCache(videoID)
	if err != nil || video == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	http.Redirect(w, r, "/"+videoID, http.StatusFound)
}
