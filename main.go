package main

import (
    "bufio"
    "encoding/hex"
    "fmt"
    "html"
    "io/ioutil"
    "log"
    "mime"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"

    "crypto/md5"
    "github.com/Seklfreak/goinside"
    . "github.com/gorilla/feeds"
    "gopkg.in/ini.v1"
)

var (
    BoardList                 []string
    TargetFolder              string
    RegexpUrl                 *regexp.Regexp
    ImageProxy                string
    SafeLinksFilename         string
    ImageCacheEnabled         bool
    ImageCacheLocation        string
    ImageCachePublicUrl       string
    ImageCacheDeleteAfterDays int
)

const (
    MaxConcurrentBoardRequests   = 2
    MaxConcurrentArticleRequests = 4
    MaxConcurrentImageDownloads  = 4
)

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
        cfg.Section("general").NewKey("feed image proxy", "\"https://your.domain/dcinside-feed-go/imageproxy.php?url=\"")
        cfg.Section("general").NewKey("safe links file", "C:\\Users\\user\\Documents\\dcinside-feed-go\\safelinks.txt")
        cfg.Section("general").NewKey("socks4 proxy", "127.0.0.1:9050")
        cfg.Section("image cache").NewKey("enabled", "false")
        cfg.Section("image cache").NewKey("location", "C:\\Users\\user\\Documents\\dcinside-feed-go\\image-cache")
        cfg.Section("image cache").NewKey("cache public url", "https://your.domain/dcinside-feed-go/image-cache/")
        cfg.Section("image cache").NewKey("delete after days", "3")
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
    ImageCacheEnabled = false
    if cfg.Section("image cache").HasKey("enabled") {
        if cfg.Section("image cache").Key("enabled").MustBool() == true &&
            cfg.Section("image cache").HasKey("location") &&
            cfg.Section("image cache").HasKey("cache public url") &&
            cfg.Section("image cache").HasKey("delete after days") {
            ImageCacheEnabled = true
            ImageCacheLocation = cfg.Section("image cache").Key("location").String()
            ImageCachePublicUrl = cfg.Section("image cache").Key("cache public url").String()
            ImageCacheDeleteAfterDays = cfg.Section("image cache").Key("delete after days").MustInt()
            err := os.MkdirAll(ImageCacheLocation, 0755)
            if err != nil {
                log.Println(err)
            }
            // remove old files
            maxAge := float64(ImageCacheDeleteAfterDays * 24)
            files, _ := ioutil.ReadDir(ImageCacheLocation)
            for _, f := range files {
                if f.IsDir() == false {
                    if time.Since(f.ModTime()).Hours() > maxAge {
                        log.Printf("removing file %s from cache because it's too old", f.Name())
                        os.Remove(ImageCacheLocation + string(filepath.Separator) + f.Name())
                    }
                }
            }
        }
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

func imageProxyUrl(imageUrl string) string {
    if SafeLinksFilename != "" {
        hasher := md5.New()
        hasher.Write([]byte(imageUrl))
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
    return ImageProxy + url.QueryEscape(imageUrl)
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

func cacheImage(semCacheImage <-chan bool, cCachedImageUrl chan<- []string, imageUrl string) {
    defer func() { <-semCacheImage }()

    client := &http.Client{}
    req, _ := http.NewRequest("GET", imageUrl, nil)
    req.Header.Set("Referer", "http://www.dcinside.com")
    response, err := client.Do(req)
    if err != nil {
        log.Println(err)
        cCachedImageUrl <- []string{imageUrl}
        return
    }
    defer response.Body.Close()

    filename := ""
    var contentLength int64
    for key, value := range response.Header {
        if key == "Content-Disposition" {
            _, params, err := mime.ParseMediaType(value[0])
            filename = params["filename"]
            if err != nil {
                log.Println(err)
                cCachedImageUrl <- []string{imageUrl}
                return
            }
        }
        if key == "Content-Length" {
            contentLength, err = strconv.ParseInt(value[0], 10, 64)
            if err != nil {
                log.Println(err)
            }
        }
    }
    if filename == "" {
        log.Printf("unable to gather filename for url: %s", imageUrl)
        cCachedImageUrl <- []string{imageUrl}
        return
    }

    completePath := ImageCacheLocation + string(os.PathSeparator) + filename

    if fileStat, err := os.Stat(completePath); err == nil {
        if fileStat.Size() == contentLength { // File already downloaded
            newImageUrl := fmt.Sprintf("%s%s", ImageCachePublicUrl, filename)
            cCachedImageUrl <- []string{imageUrl, newImageUrl}
            return
        }
        tmpPath := completePath
        i := 1
        for {
            completePath = tmpPath[0:len(tmpPath)-len(filepath.Ext(tmpPath))] +
                "-" + strconv.Itoa(i) + filepath.Ext(tmpPath)
            if _, err := os.Stat(completePath); os.IsNotExist(err) {
                break
            }
            i = i + 1
        }
    }

    bodyOfResp, err := ioutil.ReadAll(response.Body)
    if err != nil {
        log.Println(err)
        cCachedImageUrl <- []string{imageUrl}
        return
    }

    err = ioutil.WriteFile(completePath, bodyOfResp, 0644)
    if err != nil {
        log.Println(err)
        cCachedImageUrl <- []string{imageUrl}
        return
    }

    newImageUrl := fmt.Sprintf("%s%s", ImageCachePublicUrl, filename)
    cCachedImageUrl <- []string{imageUrl, newImageUrl}
}

func addArticleToFeed(semArticleToFeed <-chan bool, item *goinside.ListItem, feed *Feed) {
    defer func() { <-semArticleToFeed }()

    article := fetchArticle(item)

    content := article.Content

    content = strings.Replace(content, "&amp;", "&", -1) // temporary fix TODO: better solution
    content = strings.Replace(content, "&amp;", "&", -1) // temporary fix TODO: better solution
    content = strings.Replace(content, "&amp;", "&", -1) // temporary fix TODO: better solution

    imageUrls := RegexpUrl.FindAllString(content, -1)

    if ImageCacheEnabled == true {
        semCacheImage := make(chan bool, MaxConcurrentImageDownloads)
        cCachedImageUrl := make(chan []string, len(imageUrls))
        for _, imageUrl := range imageUrls { // Start image downloads
            semCacheImage <- true
            go cacheImage(semCacheImage, cCachedImageUrl, imageUrl)
        }
        for i := 0; i < cap(semCacheImage); i++ { // Wait for all downloads to finish
            semCacheImage <- true
        }
        for i := 0; i < cap(cCachedImageUrl); i++ { // Write new cached urls to feed content
            cachedImageUrl := <-cCachedImageUrl
            if len(cachedImageUrl) == 2 {
                content = strings.Replace(content, cachedImageUrl[0], cachedImageUrl[1], 1)
            }
            if len(cachedImageUrl) == 1 && ImageProxy != "" {
                content = strings.Replace(content, cachedImageUrl[0], imageProxyUrl(cachedImageUrl[0]), 1)
            }
        }
    } else if ImageProxy != "" {
        for _, imageUrl := range imageUrls {
            content = strings.Replace(content, imageUrl, imageProxyUrl(imageUrl), 1)
        }
    }

    feed.Add(&Item{
        Title:       article.Subject,
        Link:        &Link{Href: article.URL},
        Description: html.UnescapeString(content),
        Author:      &Author{Name: article.Name},
        Created:     article.Date,
        Id:          article.URL,
    })
}

func fetchArticle(item *goinside.ListItem) *goinside.Article {
    article, err := item.Fetch()
    if err != nil {
        log.Fatal(err)
    }
    return article
}
