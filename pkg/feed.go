package pkg

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/feeds"
	"go.uber.org/zap"
)

var alphaNumeric = regexp.MustCompile(`^[a-zA-Z0-9_]*$`)

func FeedHandler(w http.ResponseWriter, r *http.Request) {
	boards := r.URL.Query()["board"]
	if len(boards) <= 0 {
		http.Error(w, "no board specified", http.StatusBadRequest)
		return
	}

	board := boards[0]

	if board == "" || !alphaNumeric.MatchString(board) {
		http.Error(w, "invalid board name specified", http.StatusBadRequest)
		return
	}

	posts, err := ginsideClient.BoardPosts(r.Context(), board, true)
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

	for _, post := range posts {
		if len(feed.Items) > 5 {
			break
		}

		item := &feeds.Item{
			Link: &feeds.Link{
				Href: post.URL,
			},
			Author: &feeds.Author{
				Name: post.Author,
			},
			Created: post.Date,
		}

		item.Title = post.Title
		if post.Subject != "" {
			item.Title = fmt.Sprintf("[%s] %s", post.Subject, item.Title)
		}

		details, err := ginsideClient.PostDetails(r.Context(), post.URL)
		if err != nil {
			logger.Warn("error querying post details", zap.Error(err))
		} else {
			item.Content = details.ContentHTML
			if len(details.Attachments) > 0 && item.Content != "" {
				item.Content += "<br/>"
			}
			for _, attachment := range details.Attachments {
				item.Content += fmt.Sprintf("<a href=\"%s\">Download %s</a><br/>", attachment.URL, attachment.Filename)
			}
		}

		feed.Items = append(feed.Items, item)
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
