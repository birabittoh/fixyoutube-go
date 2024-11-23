package main

import (
	"net/http"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

var (
	apiKey      string
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
	err := godotenv.Load()
	if err != nil {
		logger.Info("No .env file provided.")
	}

	if os.Getenv("DEBUG") != "" {
		logger.SetLevel(logrus.DebugLevel)
		logger.Debug("Debug mode enabled (rate limiting is disabled)")
		debugSwitch = true
	}

	apiKey = getenvDefault("API_KEY", "itsme")
	port := getenvDefault("PORT", "3000")
	burstTokens := getenvDefaultParse("BURST_TOKENS", "3")
	rateLimit := getenvDefaultParse("RATE_LIMIT", "1")

	r := http.NewServeMux()
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/clear", clearHandler)
	r.HandleFunc("/watch", watchHandler)
	r.HandleFunc("/proxy/{videoId}", proxyHandler)
	r.HandleFunc("/{videoId}", shortHandler)

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
