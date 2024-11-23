package main

import (
	"bytes"
	"embed"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"text/template"

	"github.com/birabittoh/fixyoutube-go/invidious"
)

const templatesDirectory = "templates/"

var (
	//go:embed templates/index.html templates/video.html
	templates     embed.FS
	indexTemplate = template.Must(template.ParseFS(templates, templatesDirectory+"index.html"))
	videoTemplate = template.Must(template.ParseFS(templates, templatesDirectory+"video.html"))
	// userAgentRegex = regexp.MustCompile(`(?i)bot|facebook|embed|got|firefox\/92|firefox\/38|curl|wget|go-http|yahoo|generator|whatsapp|preview|link|proxy|vkshare|images|analyzer|index|crawl|spider|python|cfnetwork|node`)
	videoRegex = regexp.MustCompile(`(?i)^[a-z0-9_-]{11}$`)
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	buf := &bytes.Buffer{}
	err := indexTemplate.Execute(buf, nil)
	if err != nil {
		logger.Error("Failed to fill index template.")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	buf.WriteTo(w)
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

	buf := &bytes.Buffer{}
	err = videoTemplate.Execute(buf, video)
	if err != nil {
		logger.Error("Failed to fill video template.")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

func watchHandler(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.String())
	if err != nil {
		logger.Error("Failed to parse URL: ", r.URL.String())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	q := u.Query()
	videoId := q.Get("v")
	videoHandler(videoId, w, r)
}

func shortHandler(w http.ResponseWriter, r *http.Request) {
	videoId := r.PathValue("videoId")
	videoHandler(videoId, w, r)
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	videoId := r.PathValue("videoId")

	vb, s := invidious.ProxyVideoId(videoId)
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
