package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/PuerkitoBio/goquery"
	"github.com/jiandahao/goscrapy"
	"github.com/jiandahao/goutils/logger"
)

func main() {

	eng := goscrapy.NewEngine()

	eng.UseLogger(logger.NewSugaredLogger("engine", "debug"))

	eng.RegisterSipders(NewBaiduSpider())      // register all spiders here
	eng.RegisterPipelines(NewSimplePipeline()) // register all pipelines here
	eng.SetMaxCrawlingDepth(3)

	go eng.Start()

	defer eng.Stop()

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

// BaiduSpider baidu spider
type BaiduSpider struct{}

// NewBaiduSpider new baidu spider
func NewBaiduSpider() *BaiduSpider {
	return &BaiduSpider{}
}

// Name spider name
func (s *BaiduSpider) Name() string {
	return "baidu_spider"
}

// StartRequests start request
func (s *BaiduSpider) StartRequests() []*goscrapy.Request {
	header := http.Header{}
	header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_0_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/89.0.4389.82 Safari/537.36")
	return []*goscrapy.Request{
		{
			URL:    "https://www.baidu.com",
			Header: header,
		},
	}
}

// URLMatcher url matcher
func (s *BaiduSpider) URLMatcher() goscrapy.URLMatcher {
	return goscrapy.NewRegexpMatcher(`https:\/\/www\.baidu\.com`)
}

// Parse parse response from downloader
func (s *BaiduSpider) Parse(ctx *goscrapy.Context) (*goscrapy.Items, []*goscrapy.Request, error) {
	doc := ctx.Document()
	var href string
	var ok bool

	doc.Find("#lg > map > area").EachWithBreak(func(index int, s *goquery.Selection) bool {
		href, ok = s.Attr("href")
		if ok {
			return false
		}
		return true
	})

	var newReqs []*goscrapy.Request
	header := http.Header{}
	header.Add("Content-Type", "application/json")
	header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_0_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/89.0.4389.82 Safari/537.36")
	newReqs = append(newReqs, []*goscrapy.Request{
		{
			URL:    "https://www.baidu.com",
			Header: header,
		},
	}...)

	items := goscrapy.NewItems("baidu_href")
	items.Store("href", href)
	fmt.Println("Got:", href)
	return items, newReqs, nil
}

// SimplePipeline a simple pipeline
type SimplePipeline struct{}

// NewSimplePipeline new a simple pipline
func NewSimplePipeline() *SimplePipeline {
	return &SimplePipeline{}
}

// Name returns pipeline's name, it's the identity of pipeline, make sure every
// pipeline has it's own unique name
func (sp *SimplePipeline) Name() string {
	return "simple_pipeline"
}

// ItemList declares all items that this pipeline interested.
func (sp *SimplePipeline) ItemList() []string {
	return []string{"baidu_href"}
}

// Handle handle items
func (sp *SimplePipeline) Handle(item *goscrapy.Items) error {
	if item == nil {
		return nil
	}

	fmt.Printf("pipeline %s handles item %s\n", sp.Name(), item.Name())
	return nil
}
