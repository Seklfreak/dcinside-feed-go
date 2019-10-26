package pkg

import (
	"net/http"
	"time"

	"github.com/Seklfreak/ginside"
	"go.uber.org/zap"
)

var (
	httpClient = &http.Client{
		Timeout: 60 * time.Second,
	}
	ginsideClient = ginside.NewGInside(httpClient)
	logger        *zap.Logger
)

func init() {
	var err error

	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}
