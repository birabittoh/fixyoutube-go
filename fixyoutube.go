package main

import (
	"net/http"
	"os"
	"strconv"

	"github.com/birabittoh/fixyoutube-go/invidious"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

var (
	debugSwitch = false
	logger      = logrus.New()
)

func limit(limiter *rate.Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			status := http.StatusTooManyRequests
			http.Error(w, http.StatusText(status), status)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getEnvDefault(key string, def string) string {
	res := os.Getenv(key)
	if res == "" {
		return def
	}
	return res
}

func getEnvDefaultParse(key string, def string) float64 {
	value := getEnvDefault(key, def)
	res, err := strconv.ParseFloat(value, 64)
	if err != nil {
		logger.Fatal(err)
	}
	return res
}

func main() {
	err := godotenv.Load()
	if err != nil {
		logger.Info("No .env file provided.")
	}

	logLevel, err := logrus.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)

	if logLevel == logrus.DebugLevel {
		logger.SetLevel(logrus.DebugLevel)
		logger.Debug("Debug mode enabled (rate limiting is disabled)")
		debugSwitch = true
	}

	invidious.Init(logLevel, os.Getenv("INSTANCE"))

	port := getEnvDefault("PORT", "3000")
	burstTokens := getEnvDefaultParse("BURST_TOKENS", "3")
	rateLimit := getEnvDefaultParse("RATE_LIMIT", "1")

	adminUser = getEnvDefault("ADMIN_USER", "admin")
	adminPass = getEnvDefault("ADMIN_PASS", "admin")

	if adminUser == "admin" && adminPass == "admin" {
		logger.Println("Admin credentials not set. Please set APP_ADMIN_USER and APP_ADMIN_PASS.")
	}

	r := http.NewServeMux()
	r.HandleFunc("GET /", indexHandler)
	r.HandleFunc("GET /watch", watchHandler)
	r.HandleFunc("GET /shorts/{videoID}", shortHandler)
	r.HandleFunc("GET /proxy/{videoID}", proxyHandler)
	r.HandleFunc("GET /sub/{videoID}/{language}", subHandler)
	r.HandleFunc("GET /refresh/{videoID}", refreshHandler)
	r.HandleFunc("GET /cache", cacheHandler)
	r.HandleFunc("GET /{videoID}", shortHandler)

	r.HandleFunc("POST /download", downloadHandler)

	var serveMux http.Handler
	if debugSwitch {
		serveMux = r
	} else {
		limiter := rate.NewLimiter(rate.Limit(rateLimit), int(burstTokens))
		serveMux = limit(limiter, r)
	}
	logger.Info("Serving on port ", port)
	http.ListenAndServe(":"+port, serveMux)
}
