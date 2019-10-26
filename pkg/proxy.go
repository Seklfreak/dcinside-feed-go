package pkg

import (
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
)

var headerWhitelist = map[string]bool{
	"Last-Modified": true,
}

var dcinsideImageURL = regexp.MustCompile(`^http(s)?:\/\/([a-z0-9]+\.)?dcinside\.(com|co\.kr)\/.+$`)

func ProxyHandler(w http.ResponseWriter, r *http.Request) {
	links := r.URL.Query()["url"]
	if len(links) <= 0 {
		http.Error(w, "no url specified", http.StatusBadRequest)
		return
	}

	link := links[0]

	if link == "" || !dcinsideImageURL.MatchString(link) {
		http.Error(w, "invalid url specified", http.StatusBadRequest)
		return
	}

	fmt.Println(link)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, link, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:70.0) Gecko/20100101 Firefox/70.0")
	req.Header.Set("Referer", "https://gall.dcinside.com/")

	resp, err := httpClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusFailedDependency)
		return
	}
	defer resp.Body.Close()
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var mimeValue, filename string

	for respHeaderKey, respHeaderValues := range resp.Header {
		if len(respHeaderValues) <= 0 {
			continue
		}

		if respHeaderKey == "Content-Disposition" {
			_, params, err := mime.ParseMediaType(respHeaderValues[0])
			if err == nil && params["filename"] != "" {
				filename = params["filename"]
				mimeValue = mime.TypeByExtension(filepath.Ext(params["filename"]))
			}
		}

		if !headerWhitelist[respHeaderKey] {
			continue
		}

		w.Header().Set(respHeaderKey, respHeaderValues[0])
	}

	if mimeValue == "" {
		mimeValue = http.DetectContentType(respData)
	}

	if mimeValue != "" {
		w.Header().Set("Content-Type", mimeValue)
	}

	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": filename}))

	w.WriteHeader(resp.StatusCode)
	_, err = w.Write(respData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
