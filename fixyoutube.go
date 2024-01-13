package main

import (
	"bytes"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/BiRabittoh/fixyoutube-go/invidious"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

const templatesDirectory = "templates/"

var logger = logrus.New()
var indexTemplate = template.Must(template.ParseFiles(templatesDirectory + "index.html"))
var videoTemplate = template.Must(template.ParseFiles(templatesDirectory + "video.html"))
var userAgentRegex = regexp.MustCompile(`(?i)bot|facebook|embed|got|firefox\/92|firefox\/38|curl|wget|go-http|yahoo|generator|whatsapp|preview|link|proxy|vkshare|images|analyzer|index|crawl|spider|python|cfnetwork|node`)
var videoRegex = regexp.MustCompile(`(?i)^[a-z0-9_-]{11}$`)

var apiKey string

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
	userAgent := r.UserAgent()
	res := userAgentRegex.MatchString(userAgent)
	if !res {
		logger.Debug("Regex did not match. Redirecting. UA:", userAgent)
		url := "https://www.youtube.com/watch?v=" + videoId
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
		vars := mux.Vars(r)
		videoId := vars["videoId"]
		videoHandler(videoId, invidiousClient, w, r)
	}
}

func proxyHandler(invidiousClient *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		videoId := vars["videoId"]

		video, err := invidious.GetVideoDB(videoId)
		if err != nil {
			logger.Warn("Cannot proxy a video that is not cached: https://youtu.be/", videoId)
			http.Error(w, "Something went wrong.", http.StatusBadRequest)
			return
		}

		fmtAmount := len(video.Formats)

		s := http.StatusNotFound
		for i := fmtAmount - 1; i >= 0; i-- {
			url := video.Formats[i].Url
			logger.Debug(url)
			b, l, httpStatus := invidiousClient.ProxyVideo(url)
			if httpStatus == http.StatusOK {
				h := w.Header()
				h.Set("Status", "200")
				h.Set("Content-Type", "video/mp4")
				h.Set("Content-Length", strconv.FormatInt(l, 10))
				io.Copy(w, b)
				return
			}
			s = httpStatus
			logger.Debug("Format ", i, "failed with status code ", s)
		}

		logger.Error("proxyHandler() failed. Final code: ", s)
		http.Error(w, "Something went wrong.", s)
	}
}

func main() {
	//logger.SetLevel(logrus.DebugLevel)
	err := godotenv.Load()
	if err != nil {
		logger.Info("No .env file provided.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	apiKey = os.Getenv("API_KEY")
	if apiKey == "" {
		apiKey = "itsme"
	}

	myClient := &http.Client{Timeout: 10 * time.Second}
	videoapi := invidious.NewClient(myClient)

	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/clear", clearHandler)
	r.HandleFunc("/watch", watchHandler(videoapi))
	r.HandleFunc("/proxy/{videoId}", proxyHandler(videoapi))
	r.HandleFunc("/{videoId}", shortHandler(videoapi))
	/*
		// native go implementation
		r := http.NewServeMux()
		r.HandleFunc("/watch", watchHandler(videoapi))
		r.HandleFunc("/{videoId}/", shortHandler(videoapi))
		r.HandleFunc("/", indexHandler)
	*/
	logger.Info("Serving on port ", port)
	http.ListenAndServe(":"+port, r)
}
