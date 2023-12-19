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
	"time"

	"github.com/BiRabittoh/fixyoutube-go/invidious"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

var templatesDirectory = "templates/"
var indexTemplate = template.Must(template.ParseFiles(templatesDirectory + "index.html"))
var videoTemplate = template.Must(template.ParseFiles(templatesDirectory + "video.html"))
var blacklist = []string{"favicon.ico", "robots.txt"}
var userAgentRegex = regexp.MustCompile(`(?i)bot|facebook|embed|got|firefox\/92|firefox\/38|curl|wget|go-http|yahoo|generator|whatsapp|preview|link|proxy|vkshare|images|analyzer|index|crawl|spider|python|cfnetwork|node`)
var apiKey string

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

func videoHandler(videoId string, invidiousClient *invidious.Client, w http.ResponseWriter, r *http.Request) {
	userAgent := r.UserAgent()
	res := userAgentRegex.MatchString(userAgent)
	if !res {
		log.Println("Regex did not match. Redirecting. UA:", userAgent)
		url := "https://www.youtube.com/watch?v=" + videoId
		http.Redirect(w, r, url, http.StatusMovedPermanently)
		return
	}

	video, err := invidiousClient.GetVideo(videoId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

		videoId := u.Query().Get("v")
		videoHandler(videoId, invidiousClient, w, r)
	}
}

func shortHandler(invidiousClient *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		videoId := mux.Vars(r)["videoId"]

		if slices.Contains(blacklist, videoId) {
			http.Error(w, "Not a valid ID.", http.StatusBadRequest)
			return
		}

		videoHandler(videoId, invidiousClient, w, r)
	}
}

func proxyHandler(invidiousClient *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		videoId := mux.Vars(r)["videoId"]

		invidiousClient.ProxyVideo(videoId, w)
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
	r.HandleFunc("/{videoId}", shortHandler(videoapi))
	r.HandleFunc("/proxy/{videoId}", proxyHandler(videoapi))
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
