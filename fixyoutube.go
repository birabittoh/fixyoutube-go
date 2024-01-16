package main

import (
	"bytes"
	"embed"
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

var (
	//go:embed templates/index.html templates/video.html
	templates      embed.FS
	apiKey         string
	logger         = logrus.New()
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
		vars := mux.Vars(r)
		videoId := vars["videoId"]
		videoHandler(videoId, invidiousClient, w, r)
	}
}

func proxyHandler(invidiousClient *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		videoId := vars["videoId"]

		vb, s := invidiousClient.ProxyVideoId(videoId)
		if s != http.StatusOK {
			logger.Error("proxyHandler() failed. Final code: ", s)
			http.Error(w, "Something went wrong.", s)
			return
		}
		if !vb.ValidateLength() {
			logger.Error("Buffer length is inconsistent.")
			http.Error(w, "Something went wrong.", http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("Status", "200")
		h.Set("Content-Type", "video/mp4")
		h.Set("Content-Length", strconv.FormatInt(vb.Length, 10))
		io.Copy(w, vb.Buffer)
	}
}

func getenvDefault(key string, def string) string {
	res := os.Getenv(key)
	if res == "" {
		return def
	}
	return res
}

func getenvDefaultParse(key string, def string) float64 {
	value := getenvDefault(key, def)
	res, err := strconv.ParseFloat(value, 64)
	if err != nil {
		logger.Fatal(err)
	}
	return res
}

func main() {
	logger.SetLevel(logrus.DebugLevel)
	err := godotenv.Load()
	if err != nil {
		logger.Info("No .env file provided.")
	}

	apiKey = getenvDefault("API_KEY", "itsme")
	port := getenvDefault("PORT", "3000")
	cacheDuration := getenvDefaultParse("CACHE_DURATION_MINUTES", "5")
	timeoutDuration := getenvDefaultParse("TIMEOUT_DURATION_MINUTES", "10")
	maxSizeMB := getenvDefaultParse("MAX_SIZE_MB", "20")

	myClient := &http.Client{Timeout: 10 * time.Second}
	options := invidious.ClientOptions{
		CacheDuration:   time.Duration(cacheDuration) * time.Minute,
		TimeoutDuration: time.Duration(timeoutDuration) * time.Minute,
		MaxSizeBytes:    int64(maxSizeMB * 1000000),
	}
	videoapi := invidious.NewClient(myClient, options)

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
