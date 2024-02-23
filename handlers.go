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

	"github.com/BiRabittoh/fixyoutube-go/invidious"
)

const templatesDirectory = "templates/"

var (
	//go:embed templates/index.html templates/video.html
	templates      embed.FS
	indexTemplate  = template.Must(template.ParseFS(templates, templatesDirectory+"index.html"))
	videoTemplate  = template.Must(template.ParseFS(templates, templatesDirectory+"video.html"))
	userAgentRegex = regexp.MustCompile(`(?i)bot|facebook|embed|got|firefox\/92|firefox\/38|curl|wget|go-http|yahoo|generator|whatsapp|preview|link|proxy|vkshare|images|analyzer|index|crawl|spider|python|cfnetwork|node`)
	videoRegex     = regexp.MustCompile(`(?i)^[a-z0-9_-]{11}$`)
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

func clearHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
		return
	}

	err := r.ParseForm()
	if err != nil {
		logger.Error("Failed to parse form in /clear.")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	providedKey := r.PostForm.Get("apiKey")
	if providedKey != apiKey {
		logger.Debug("Wrong API key: ", providedKey)
		http.Error(w, "Wrong or missing API key.", http.StatusForbidden)
		return
	}

	invidious.ClearDB()
	logger.Info("Cache cleared.")
	http.Error(w, "Done.", http.StatusOK)
}

func videoHandler(videoId string, invidiousClient *invidious.Client, w http.ResponseWriter, r *http.Request) {
	url := "https://www.youtube.com/watch?v=" + videoId
	userAgent := r.UserAgent()
	res := userAgentRegex.MatchString(userAgent)
	if !res {
		logger.Debug("Regex did not match. Redirecting. UA:", userAgent)
		http.Redirect(w, r, url, http.StatusMovedPermanently)
		return
	}

	if !videoRegex.MatchString(videoId) {
		logger.Info("Invalid video ID: ", videoId)
		http.Error(w, "Invalid video ID.", http.StatusBadRequest)
		return
	}

	video, err := invidiousClient.GetVideo(videoId, true)
	if err != nil {
		logger.Info("Wrong video ID: ", videoId)
		http.Error(w, "Wrong video ID.", http.StatusNotFound)
		return
	}

	if video.Url == "" {
		logger.Debug("No URL available. Redirecting.")
		http.Redirect(w, r, url, http.StatusMovedPermanently)
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

func watchHandler(invidiousClient *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := url.Parse(r.URL.String())
		if err != nil {
			logger.Error("Failed to parse URL: ", r.URL.String())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		q := u.Query()
		videoId := q.Get("v")
		videoHandler(videoId, invidiousClient, w, r)
	}
}

func shortHandler(invidiousClient *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		videoId := r.PathValue("videoId")
		videoHandler(videoId, invidiousClient, w, r)
	}
}

func proxyHandler(invidiousClient *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		videoId := r.PathValue("videoId")

		vb, s := invidiousClient.ProxyVideoId(videoId)
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
}
