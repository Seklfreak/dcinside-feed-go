package pkg

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/feeds"
)

var alphaNumeric = regexp.MustCompile("^[a-zA-Z0-9_]*$")

func Handler(w http.ResponseWriter, r *http.Request) {
	boards := r.URL.Query()["board"]
	if len(boards) <= 0 {
		http.Error(w, "no board specified", http.StatusBadRequest)
		return
	}

	board := boards[0]

	if !alphaNumeric.MatchString(board) {
		http.Error(w, "invalid board name specified", http.StatusBadRequest)
		return
	}

	posts, err := getClient().BoardPosts(r.Context(), board, true)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected status code: 404") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusFailedDependency)
		return
	}

	feed := &feeds.Feed{
		Title: fmt.Sprintf("gall.dcinside.com: %s", board),
		Link: &feeds.Link{
			Href: fmt.Sprintf("https://gall.dcinside.com/board/lists/?id=%s", board),
		},
	}

	var title string
	for _, post := range posts {
		title = post.Title
		if post.Subject != "" {
			title = fmt.Sprintf("[%s] %s", post.Subject, title)
		}

		feed.Items = append(feed.Items, &feeds.Item{
			Title: title,
			Link: &feeds.Link{
				Href: post.URL,
			},
			Author: &feeds.Author{
				Name: post.Author,
			},
			Created: post.Date,
		})
	}

	w.Header().Set("Content-Type", "application/atom+xml")
	atom, err := feed.ToAtom()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = fmt.Fprint(w, atom)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
