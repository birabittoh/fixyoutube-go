package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"time"

	"github.com/BiRabittoh/fixyoutube-go/invidious"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

var templatesDirectory = "templates/"
var indexTemplate = template.Must(template.ParseFiles(templatesDirectory + "index.html"))
var videoTemplate = template.Must(template.ParseFiles(templatesDirectory + "video.html"))

func indexHandler(w http.ResponseWriter, r *http.Request) {
	buf := &bytes.Buffer{}
	err := indexTemplate.Execute(buf, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	buf.WriteTo(w)
}

func watchHandler(newsapi *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := url.Parse(r.URL.String())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		params := u.Query()
		videoId := params.Get("v")

		video, err := newsapi.FetchEverything(videoId)
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
}

var blacklist = []string{"favicon.ico", "robots.txt"}

func shortHandler(newsapi *invidious.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		videoId := mux.Vars(r)["videoId"]

		if slices.Contains(blacklist, videoId) {
			http.Error(w, "Not a valid ID.", http.StatusBadRequest)
			return
		}

		video, err := newsapi.FetchEverything(videoId)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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
	videoapi := invidious.NewClient(myClient, "y.birabittoh.duckdns.org")

	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/watch", watchHandler(videoapi))
	r.HandleFunc("/{videoId}", shortHandler(videoapi))
	//r.HandleFunc("/proxy/{videoId}", proxyHandler)

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
