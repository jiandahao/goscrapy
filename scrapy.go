package goscrapy

import (
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

// RequestHandleFunc request handler func
type RequestHandleFunc func(*Request) error

// Request represents crawling request
type Request struct {
	Method string      `json:"method,omitempty"`
	URL    string      `json:"url,omitempty"`
	Header http.Header `json:"header,omitempty"`
	Query  url.Values  `json:"query,omitempty"`
	// using to decide scheduling sequence. It only means something when using a 
	// scheduler that schedules requests based on request weight.
	Weight int         

	// private fields
	currentDepth int  // current request depth
	aborted      bool // true if request has been aborted
	ctxMap       map[string]interface{}
}

// Abort aborts current request, you could use it at your request middleware
// to make sure certain request will not be handled by downloader and spiders.
func (r *Request) Abort() {
	r.aborted = true
}

// IsAborted returns true if the current request was aborted.
func (r *Request) IsAborted() bool {
	return r.aborted
}

// WithContextValue sets the value into request associated with the key.
func (r *Request) WithContextValue(key string, value interface{}) {
	if r.ctxMap == nil {
		r.ctxMap = make(map[string]interface{})
	}

	r.ctxMap[key] = value
}

// ContextValue returns the value associated with this request for key,
// or nil if no value is associated with key.
func (r *Request) ContextValue(key string) interface{} {
	if r.ctxMap == nil {
		return nil
	}

	val, ok := r.ctxMap[key]
	if !ok {
		return nil
	}

	return val
}

// ResponseHandleFunc response handler func
type ResponseHandleFunc func(*Response) error

// Response represents crawling response
type Response struct {
	Status     string `json:"status,omitempty"`      // e.g. "200 OK"
	StatusCode int    `json:"status_code,omitempty"` // e.g. 200
	// Request represents request that was send to obtain this response.
	Request *Request `json:"request,omitempty"`
	// Document represents an HTML document to be manipulated.
	Document *goquery.Document `json:"-"`
	// Body represents the response body.
	Body io.ReadCloser `json:"-"`
	// ContentLength records the length of the associated content. more details see http.Response.
	ContentLength int64 `json:"content_length,omitempty"`
	// Header represents response header, maps header keys to values.
	Header http.Header `json:"header,omitempty"`
}

// Context represents the scraping and crawling context
type Context struct {
	response *Response
}

// Response returns the downloading response
func (ctx *Context) Response() *Response {
	return ctx.response
}

// Request returns the crawling request
func (ctx *Context) Request() *Request {
	return ctx.response.Request
}

// Document returns HTML document
func (ctx *Context) Document() *goquery.Document {
	if ctx.response == nil {
		return nil
	}
	return ctx.Response().Document
}

// Items items
type Items struct {
	sync.Map
	name string
}

// NewItems new items with specified name, goscrapy pipeline will
// make the decision whether to handle an item based on the item's name.
func NewItems(name string) *Items {
	if name == "" {
		panic("invalid item name")
	}
	return &Items{
		Map:  sync.Map{},
		name: name,
	}
}

// Name returns items' name
func (item *Items) Name() string {
	return item.name
}

// Spider is an interface that defines how a certain site (or a group of sites) will be scraped,
// including how to perform the crawl (i.e. follow links) and how to extract structured data
// from their pages (i.e. scraping items). In other words, Spiders are the place where you define
// the custom behavior for crawling and parsing pages for a particular site (or, in some cases, a group of sites).
// For spiders, the scraping cycle goes through something like this:
// 1. Using initial Requests generated by StartRequests to crawl the first URLs.
// 2. Parsing the response (web page), then return items object (structured data) and request objects. Those requests
//    will be added into scheduler by goscrapy engine and downloaded by downloader in the future.
type Spider interface {
	Name() string
	StartRequests() []*Request
	URLMatcher() URLMatcher
	Parse(ctx *Context) (*Items, []*Request, error)
}

// Pipeline pipeline
type Pipeline interface {
	Name() string       // returns pipeline's name
	ItemList() []string // returns all items' name that this pipeline cares about
	Handle(items *Items) error
}
