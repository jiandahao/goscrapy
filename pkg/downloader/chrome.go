package downloader

import (
	"bytes"
	"fmt"

	"net/http"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/jiandahao/goscrapy"
	"github.com/jiandahao/goscrapy/pkg/broswer/chrome"
	"github.com/tebeka/selenium"
	"go.uber.org/zap"
)

var _ goscrapy.Downloader = &ChromeDownloader{}

// ChromeDownloader downloader based on chrome.
type ChromeDownloader struct {
	chromeCfg chrome.Config
	// chromeEngine *chrome.Broswer
	cache Cache

	sync.Mutex

	broswer *chrome.Broswer
}

// Cache cache
type Cache interface {
	Store(*goscrapy.Response) error
	Load(*goscrapy.Request) (*goscrapy.Response, error)
}

// NewChromeDownloader creates a downloader instance.
func NewChromeDownloader(cfg chrome.Config) (*ChromeDownloader, error) {
	broswer, err := chrome.NewBroswer(cfg)
	if err != nil {
		return nil, err
	}

	if err := broswer.Open(); err != nil {
		return nil, err
	}

	return &ChromeDownloader{
		chromeCfg: cfg,
		broswer:   broswer,
	}, nil
}

// SetCache set cache
func (cd *ChromeDownloader) SetCache(c Cache) {
	cd.cache = c
}

func (cd *ChromeDownloader) getFromCache(req *goscrapy.Request) *goscrapy.Response {
	if cd.cache == nil {
		return nil
	}

	resp, err := cd.cache.Load(req)
	if err != nil {
		fmt.Printf("failed tp load cache: %v \n", err)
		return nil
	}

	return resp
}

func (cd *ChromeDownloader) setCache(resp *goscrapy.Response) {
	if cd.cache == nil {
		return
	}

	err := cd.cache.Store(resp)
	if err != nil {
		fmt.Printf("failed to set cache: %v\n", err)
	}
}

// Download downloads
func (cd *ChromeDownloader) Download(req *goscrapy.Request) (*goscrapy.Response, error) {
	if resp := cd.getFromCache(req); resp != nil {
		return resp, nil
	}

	cd.Lock()
	defer cd.Unlock()
	fmt.Printf("start to fetch page: %s\n", req.URL)

	if err := cd.retryer(10, func() error {
		if err := cd.broswer.Get(req.URL); err != nil {
			return err
		}

		return nil
	}); err != nil {
		fmt.Printf("handle request failed: %v, %v\n", zap.Any("request", req), zap.Error(err))
		return nil, err
	}

	elem, err := cd.broswer.WebDriver.FindElement(selenium.ByXPATH, "//*")
	if err != nil {
		return nil, err
	}

	html, err := elem.GetAttribute("innerHTML")
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer([]byte(html))
	doc, err := goquery.NewDocumentFromReader(buf)
	if err != nil {
		return nil, err
	}

	resp := &goscrapy.Response{
		Request:    req,
		Document:   doc,
		Status:     "OK",
		StatusCode: http.StatusOK,
	}

	cd.setCache(resp)

	return resp, nil
}

func (cd *ChromeDownloader) retryer(n int, h func() error) error {
	handler := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("recover from chrome panic: %v", r)
			}
		}()

		return h()
	}

	var err error
	for i := 0; i < n; i++ {
		err = handler()
		if err == nil {
			break
		}

		fmt.Printf("something wrong happens,cause: %v, restarting chrome: retry (%v / %v )\n", err, i+1, n)
		cd.broswer.Close()
		cd.broswer.Open()
	}

	return err
}
