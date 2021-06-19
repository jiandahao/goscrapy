package goscrapy

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// DownloadOption downloader options
type DownloadOption struct {
	// Timeout controls the entire lifetime of a request and its response:
	// obtaining a connection, sending the request, and reading
	// the response headers and body
	Timeout time.Duration
	// HTTPClient is the client that sends an HTTP request. If you wanna add
	// proxy, implementing it in http.Transport
	HTTPClient *http.Client
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
	if opt.HTTPClient != nil {
		dd.httpClient = opt.HTTPClient
	}
}

// SetHTTPClient set http client using to fetch pages
func (dd *DefaultDownloader) SetHTTPClient(client *http.Client) {
	dd.httpClient = client
}

// Download sends http request and using goquery to get http document
func (dd *DefaultDownloader) Download(req *Request) (*Response, error) {
	ctx := context.Background()
	if dd.opt.Timeout > 0 {
		var cancle context.CancelFunc
		ctx, cancle = context.WithTimeout(ctx, dd.opt.Timeout)
		defer cancle()
	}

	r, err := dd.makeRequest(ctx, req)
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

func (dd *DefaultDownloader) makeRequest(ctx context.Context, req *Request) (*http.Request, error) {
	r, err := http.NewRequestWithContext(ctx, req.Method, req.URL, nil)
	if err != nil {
		return nil, err
	}

	r.Header = req.Header
	r.URL.RawQuery = req.Query.Encode()
	return r, nil
}
