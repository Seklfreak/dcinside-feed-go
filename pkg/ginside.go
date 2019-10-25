package pkg

import (
	"net/http"
	"sync"
	"time"

	"github.com/Seklfreak/ginside"
)

var client *ginside.GInside
var clientLock sync.Mutex

func getClient() *ginside.GInside {
	clientLock.Lock()
	defer clientLock.Unlock()

	if client != nil {
		return client
	}

	client = ginside.NewGInside(&http.Client{
		Timeout: 60 * time.Second,
	})
	return client
}
