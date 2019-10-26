package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Seklfreak/dcinside-feed-go/pkg"
)

func feedHandler(w http.ResponseWriter, r *http.Request) {
	pkg.FeedHandler(w, r)
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	pkg.ProxyHandler(w, r)
}

func main() {
	http.HandleFunc("/", feedHandler)
	http.HandleFunc("/proxy", proxyHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
