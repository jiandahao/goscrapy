package goscrapy

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// DownloadOption downloader options
type DownloadOption struct {
	Timeout time.Duration
	Delay   time.Duration
}

// Downloader is an interface that representing the ability to download
// data from internet.
type Downloader interface {
	Init(option DownloadOption)
	Download(*Request) (*Response, error)
}

// DefaultDownloader a simple downloader implementation
type DefaultDownloader struct {
	opt        DownloadOption
	httpClient *http.Client
}

// Init init default downloader
func (dd *DefaultDownloader) Init(opt DownloadOption) {
	dd.opt = opt
	dd.httpClient = http.DefaultClient
}

// Download sends http request and using goquery to get
func (dd *DefaultDownloader) Download(req *Request) (*Response, error) {
	r, err := dd.makeRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := dd.httpClient.Do(r)
	if err != nil {
		return nil, err
	}

	body := resp.Body
	defer body.Close()

	// TODO: DON'T use ioutil.ReadAll.
	data, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	resp.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	return &Response{
		Response: resp,
		Document: doc,
	}, nil
}

func (dd *DefaultDownloader) makeRequest(req *Request) (*http.Request, error) {
	r, err := http.NewRequest(req.Method, req.URL, nil)
	if err != nil {
		return nil, err
	}

	r.Header = req.Header
	r.URL.RawQuery = req.Query.Encode()
	return r, nil
}
