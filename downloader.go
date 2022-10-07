package goscrapy

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

var defaultHTTPClient = &http.Client{}

// Downloader is an interface that representing the ability to download
// data from internet. It is responsible for fetching web pages, the downloading
// response will be took over by engine, in turn, fed to spiders.
type Downloader interface {
	Download(*Request) (*Response, error)
}

// DefaultDownloader a simple downloader implementation
type DefaultDownloader struct {
	httpClient *http.Client
}

// SetHTTPClient set http client using to fetch pages
func (dd *DefaultDownloader) SetHTTPClient(client *http.Client) {
	dd.httpClient = client
}

// Download sends http request and using goquery to get http document
func (dd *DefaultDownloader) Download(req *Request) (*Response, error) {
	ctx := context.Background()

	r, err := dd.makeRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	httpClient := dd.httpClient
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}

	resp, err := httpClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data := make([]byte, 0, resp.ContentLength+512)
	buf := bytes.NewBuffer(data)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(buf.Bytes()))
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		ContentLength: resp.ContentLength,
		Request:       req,
		Document:      doc,
		Body:          buf.Bytes(),
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
