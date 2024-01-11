package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/BiRabittoh/fixyoutube-go/invidious"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

const templatesDirectory = "templates/"

var indexTemplate = template.Must(template.ParseFiles(templatesDirectory + "index.html"))
var videoTemplate = template.Must(template.ParseFiles(templatesDirectory + "video.html"))
var blacklist = []string{"favicon.ico", "robots.txt", "proxy"}
var userAgentRegex = regexp.MustCompile(`(?i)bot|facebook|embed|got|firefox\/92|firefox\/38|curl|wget|go-http|yahoo|generator|whatsapp|preview|link|proxy|vkshare|images|analyzer|index|crawl|spider|python|cfnetwork|node`)
var videoRegex = regexp.MustCompile(`^(?i)[a-z0-9_-]{11}$`)

var apiKey string

func parseFormatIndex(formatIndexString string) int {
	formatIndex, err := strconv.Atoi(formatIndexString)
	if err != nil || formatIndex < 0 {
		log.Println("Error: could not parse formatIndex.")
		return 0
	}
	return formatIndex
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	buf := &bytes.Buffer{}
	err := indexTemplate.Execute(buf, nil)
	if err != nil {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	providedKey := r.PostForm.Get("apiKey")
	if providedKey != apiKey {
		http.Error(w, "Wrong or missing API key.", http.StatusForbidden)
		return
	}

	invidious.ClearDB()
	http.Error(w, "Done.", http.StatusOK)
}

func videoHandler(videoId string, formatIndex int, invidiousClient *invidious.Client, w http.ResponseWriter, r *http.Request) {
	userAgent := r.UserAgent()
	res := userAgentRegex.MatchString(userAgent)
	if !res {
		log.Println("Regex did not match. Redirecting. UA:", userAgent)
		url := "https://www.youtube.com/watch?v=" + videoId
		http.Redirect(w, r, url, http.StatusMovedPermanently)
		return
	}

	if !videoRegex.MatchString(videoId) {
		http.Error(w, "Bad Video ID.", http.StatusBadRequest)
		return
	}

	video, err := invidiousClient.GetVideo(videoId)
	if err != nil {
		http.Error(w, "Wrong Video ID.", http.StatusNotFound)
		return
	}

	video.FormatIndex = formatIndex % len(video.Formats)

	buf := &bytes.Buffer{}
	err = videoTemplate.Execute(buf, video)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

func watchHandler(invidiousClient *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := url.Parse(r.URL.String())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		q := u.Query()
		videoId := q.Get("v")
		formatIndex := parseFormatIndex(q.Get("f"))
		videoHandler(videoId, formatIndex, invidiousClient, w, r)
	}
}

func shortHandler(invidiousClient *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		videoId := vars["videoId"]
		formatIndex := parseFormatIndex(vars["formatIndex"])

		if slices.Contains(blacklist, videoId) {
			http.Error(w, "Not a valid ID.", http.StatusBadRequest)
			return
		}

		videoHandler(videoId, formatIndex, invidiousClient, w, r)
	}
}

func proxyHandler(invidiousClient *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		videoId := vars["videoId"]
		formatIndex := parseFormatIndex(vars["formatIndex"])
		invidiousClient.ProxyVideo(w, videoId, formatIndex)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file provided.")
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
	r.HandleFunc("/proxy/{videoId}/{formatIndex}", proxyHandler(videoapi))
	r.HandleFunc("/{videoId}", shortHandler(videoapi))
	r.HandleFunc("/{videoId}/{formatIndex}", shortHandler(videoapi))
	/*
		// native go implementation (useless until february 2024)
		r := http.NewServeMux()
		r.HandleFunc("/watch", watchHandler(videoapi))
		r.HandleFunc("/{videoId}/", shortHandler(videoapi))
		r.HandleFunc("/", indexHandler)
	*/
	println("Serving on port", port)
	http.ListenAndServe(":"+port, r)
}
