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
// data from internet. It is responsible for fetching web pages, the downloading
// response will be took over by engine, in turn, fed to spiders.
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
	dd.httpClient = &http.Client{}
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
	defer resp.Body.Close()

	buf := &bytes.Buffer{}
	if resp.ContentLength >= 0 {
		data := make([]byte, 0, resp.ContentLength+512)
		buf = bytes.NewBuffer(data)
	}

	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}

	body := ioutil.NopCloser(buf)
	doc, _ := goquery.NewDocumentFromReader(bytes.NewBuffer(buf.Bytes()))

	return &Response{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		ContentLength: resp.ContentLength,
		Request:       req,
		Document:      doc,
		Body:          body,
		Header:        resp.Header,
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
