package main

import (
	"html"
	"io/ioutil"
	"log"
	"net/url"
	"path/filepath"
	"regexp"
	"time"

	"github.com/Seklfreak/goinside"
	. "github.com/gorilla/feeds"
	"gopkg.in/ini.v1"
)

var (
	BoardList    []string
	TargetFolder string
	RegexpUrl    *regexp.Regexp
	ImageProxy   string
)

func imageProxyUrl(imageUrl []byte) []byte {
	if ImageProxy != "" {
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
		cfg.Section("general").NewKey("feed image proxy", "https://your.domain/proxy.php?url=")
		cfg.Section("general").NewKey("socks4 proxy", "127.0.0.1:9050")
		err = cfg.SaveTo("config.ini")

		if err != nil {
			log.Fatal(err)
		}
		log.Println("Wrote config file, please fill out and restart the program")
		return
	}

	if cfg.Section("general").HasKey("feed image proxy") {
		ImageProxy = cfg.Section("general").Key("feed image proxy").String()
	}

	if cfg.Section("general").HasKey("socks4 proxy") {
		goinside.Socks4 = cfg.Section("general").Key("socks4 proxy").String()
	}

	RegexpUrl, err = regexp.Compile(
		`(http:\/\/[a-z0-9]+.dcinside.com\/(viewimage(Pop)?.php\\?[^"\'\<\ ]+|[a-z0-9\-\_]+\.(jpg|jpeg|png)))`)
	if err != nil {
		log.Fatal(err)
		return
	}

	BoardList = cfg.Section("general").Key("boards").Strings(",")
	TargetFolder = cfg.Section("general").Key("feedfolder").String()
	for _, board := range BoardList {
		log.Println("checking board: " + board)
		URL := "http://gall.dcinside.com/board/lists/?id=" + board

		now := time.Now()
		feed := &Feed{
			Title:       "dcinside: " + board,
			Link:        &Link{Href: URL},
			Description: "first page of the articles on the dcinside board " + board,
			Created:     now,
		}

		list, err := goinside.FetchBestList(URL, 1)
		if err != nil {
			log.Fatalln("FetchList:", err)
		}
		for _, item := range list.Items {
			article, err := item.Fetch()
			if err != nil {
				log.Fatal(err)
			}
			/*
				var imageUrls []string
				if article.HasImage {
					imageUrls, err = item.FetchImageURLs()
					if err != nil {
						log.Fatal(err)
					}
				}
				for _, imageUrl := range imageUrls {
					log.Println(imageUrl)
				}*/
			editedContent := html.UnescapeString(string(RegexpUrl.ReplaceAllFunc([]byte(html.UnescapeString(article.Content)), imageProxyUrl)))

			feed.Add(&Item{
				Title:       article.Subject,
				Link:        &Link{Href: article.URL},
				Description: string(html.UnescapeString(editedContent)),
				Author:      &Author{Name: article.Name},
				Created:     article.Date,
				Id:          article.URL,
			})
		}

		atom, err := feed.ToRss()
		if err != nil {
			log.Fatal(err)
		}
		targetXml := TargetFolder + string(filepath.Separator) + board + ".xml"
		log.Println("Writing", board, "feed to", targetXml)
		err = ioutil.WriteFile(targetXml, []byte(atom), 0644)
		if err != nil {
			log.Fatal(err)
		}
	}
}
