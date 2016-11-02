package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"crypto/md5"
	"github.com/Seklfreak/goinside"
	. "github.com/gorilla/feeds"
	"gopkg.in/ini.v1"
)

var (
	BoardList         []string
	TargetFolder      string
	RegexpUrl         *regexp.Regexp
	ImageProxy        string
	SafeLinksFilename string
)

const (
	MaxConcurrentBoardRequests   = 2
	MaxConcurrentArticleRequests = 4
)

func stringAlreadyInFile(filename string, needle string) bool {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), needle) {
			return true
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalln(err)
	}
	return false
}

func imageProxyUrl(imageUrl []byte) []byte {
	if ImageProxy != "" {
		if SafeLinksFilename != "" {
			hasher := md5.New()
			hasher.Write(imageUrl)
			md5sum := hex.EncodeToString(hasher.Sum(nil))
			if stringAlreadyInFile(SafeLinksFilename, md5sum) == false {
				f, err := os.OpenFile(SafeLinksFilename, os.O_APPEND|os.O_WRONLY, 0666)
				if err != nil {
					log.Fatalln(err)
				}

				defer f.Close()

				if _, err = f.WriteString(md5sum + "\n"); err != nil {
					log.Fatalln(err)
				}
			}
		}
		return []byte(ImageProxy + url.QueryEscape(string(imageUrl)))
	} else {
		return []byte(string(imageUrl))
	}
}

func main() {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		log.Println(err)
		cfg = ini.Empty()
	}

	if !cfg.Section("general").HasKey("feedfolder") &&
		!cfg.Section("general").HasKey("boards") {
		cfg.Section("general").NewKey("feedfolder", "C:\\Users\\user\\Documents\\dcinside-feed-go")
		cfg.Section("general").NewKey("boards", "board1, board2, board3")
		cfg.Section("general").NewKey("feed image proxy", "\"https://your.domain/proxy.php?url=\"")
		cfg.Section("general").NewKey("safe links file", "C:\\Users\\user\\Documents\\dcinside-feed-go\\safelinks.txt")
		cfg.Section("general").NewKey("socks4 proxy", "127.0.0.1:9050")
		err = cfg.SaveTo("config.ini")

		if err != nil {
			log.Fatal(err)
		}
		log.Println("wrote config file, please fill out and restart the program")
		return
	}

	if cfg.Section("general").HasKey("feed image proxy") {
		ImageProxy = cfg.Section("general").Key("feed image proxy").String()
	}
	if cfg.Section("general").HasKey("safe links file") {
		SafeLinksFilename = cfg.Section("general").Key("safe links file").String()
		if _, err := os.Stat(SafeLinksFilename); os.IsNotExist(err) {
			f, err := os.OpenFile(SafeLinksFilename, os.O_RDONLY|os.O_CREATE, 0666)
			if err != nil {
				log.Fatalln(err)
			}
			f.Close()
			log.Printf("safe links file created: %s", SafeLinksFilename)
		}
	}

	if cfg.Section("general").HasKey("socks4 proxy") {
		goinside.Socks4 = cfg.Section("general").Key("socks4 proxy").String()
	}

	RegexpUrl, err = regexp.Compile(
		`(http:\/\/[a-z0-9]+.dcinside.com\/(viewimage(M)?(Pop)?.php\\?[^"\'\<\ ]+|[a-z0-9\-\_]+\.(jpg|jpeg|png)))`)
	if err != nil {
		log.Fatal(err)
		return
	}

	BoardList = cfg.Section("general").Key("boards").Strings(",")
	TargetFolder = cfg.Section("general").Key("feedfolder").String()

	semCreateFeeds := make(chan bool, MaxConcurrentBoardRequests)
	for _, board := range BoardList {
		semCreateFeeds <- true
		log.Printf("checking board: %s", board)
		go createFeedForBoard(semCreateFeeds, board)
	}
	for i := 0; i < cap(semCreateFeeds); i++ {
		semCreateFeeds <- true
	}
}

func createFeedForBoard(semCreateFeeds <-chan bool, board string) {
	defer func() { <-semCreateFeeds }()

	URL := "http://gall.dcinside.com/board/lists/?id=" + board
	now := time.Now()
	feed := &Feed{
		Title:       "dcinside: " + board,
		Link:        &Link{Href: URL},
		Description: "first page of the best articles on the dcinside board " + board,
		Created:     now,
	}

	list, err := goinside.FetchBestList(URL, 1)
	if err != nil {
		log.Fatalln("FetchList:", err)
	}

	semArticleToFeed := make(chan bool, MaxConcurrentArticleRequests)
	for _, item := range list.Items {
		semArticleToFeed <- true
		go addArticleToFeed(semArticleToFeed, item, feed)
	}
	for i := 0; i < cap(semArticleToFeed); i++ {
		semArticleToFeed <- true
	}

	atom, err := feed.ToRss()
	if err != nil {
		log.Fatal(err)
	}
	targetXml := TargetFolder + string(filepath.Separator) + board + ".xml"
	log.Printf("writing %s feed to: %s", board, targetXml)
	err = ioutil.WriteFile(targetXml, []byte(atom), 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func addArticleToFeed(semArticleToFeed <-chan bool, item *goinside.ListItem, feed *Feed) {
	defer func() { <-semArticleToFeed }()

	cArticle := make(chan *goinside.Article)
	cImageUrls := make(chan []string)

	go fetchArticle(cArticle, item)
	go fetchImageUrls(cImageUrls, item)

	var article *goinside.Article
	var imageUrls []string
	for i := 0; i <= 1; i++ {
		select {
		case article = <-cArticle:
		case imageUrls = <-cImageUrls:
		}
	}

	content := ""
	if len(imageUrls) > 0 {
		content += "<p><b>Embedded images:</b><br />"
		for _, imageUrl := range imageUrls {
			content += fmt.Sprintf("<a href=\"%s\" target=\"_blank\"><img src=\"%s\" /><br /></a>", imageUrl, imageUrl)
		}
		content += "</p>"
	}
	content += article.Content
	editedContent := html.UnescapeString(string(RegexpUrl.ReplaceAllFunc([]byte(html.UnescapeString(content)), imageProxyUrl)))

	feed.Add(&Item{
		Title:       article.Subject,
		Link:        &Link{Href: article.URL},
		Description: string(html.UnescapeString(editedContent)),
		Author:      &Author{Name: article.Name},
		Created:     article.Date,
		Id:          article.URL,
	})
}

func fetchArticle(channel chan<- *goinside.Article, item *goinside.ListItem) {
	article, err := item.Fetch()
	if err != nil {
		log.Fatal(err)
	}
	channel <- article
}

func fetchImageUrls(channel chan<- []string, item *goinside.ListItem) {
	if item.HasImage {
		imageUrls, err := item.FetchImageURLs()
		if err != nil {
			log.Fatal(err)
		}
		channel <- imageUrls
	} else {
		channel <- make([]string, 0)
	}
}
