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
var userAgentRegex = regexp.MustCompile(`bot|facebook|embed|got|firefox\/92|firefox\/38|curl|wget|go-http|yahoo|generator|whatsapp|preview|link|proxy|vkshare|images|analyzer|index|crawl|spider|python|cfnetwork|node`)

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
	//TODO: check with some secret API key before clearing cache.
	invidious.ClearDB()
	//TODO: return a message
}

func videoHandler(videoId string, invidiousClient *invidious.Client, w http.ResponseWriter, r *http.Request) {

	res := userAgentRegex.MatchString(r.UserAgent())
	if !res {
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

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	myClient := &http.Client{Timeout: 10 * time.Second}
	videoapi := invidious.NewClient(myClient)

	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/clear", clearHandler)
	r.HandleFunc("/watch", watchHandler(videoapi))
	r.HandleFunc("/{videoId}", shortHandler(videoapi))
	//TODO: r.HandleFunc("/proxy/{videoId}", proxyHandler)
	//TODO: check user agent before serving templates.

	/*
		UA_REGEX = r"bot|facebook|embed|got|firefox\/92|firefox\/38|curl|wget|go-http|yahoo|generator|whatsapp|preview|link|proxy|vkshare|images|analyzer|index|crawl|spider|python|cfnetwork|node"
		PROXY_HEADERS_REQUEST = { "Range": f"bytes=0-{MAX_SIZE_MB}000000" }
		PROXY_HEADERS_RESPONSE = { "Content-Type": "video/mp4" }
	*/

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
