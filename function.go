package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Seklfreak/dcinside-feed-go/pkg"
)

func handler(w http.ResponseWriter, r *http.Request) {
	pkg.Handler(w, r)
}

func main() {
	http.HandleFunc("/", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
