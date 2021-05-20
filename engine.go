package goscrapy

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jiandahao/goutils/logger"
	"github.com/jiandahao/goutils/waitgroup"
)

const (
	stateStoped = iota
	stateRunning
)

// Engine represents scraping engine, it is responsible for managing
// the data flow among scheduler, downloader and spiders.
type Engine struct {
	sched       Scheduler
	downloader  Downloader
	spiders     []Spider
	pipelines   map[string][]Pipeline
	concurrency int
	lg          logger.Logger
	mux         sync.RWMutex
	state       int
	pendingCnt  int32 // pendingCnt represents how many workers are waiting to handle request
}

// NewEngine create a new goscrapy engine
func NewEngine() *Engine {
	downloader := &DefaultDownloader{}
	downloader.Init(DownloadOption{
		Timeout: time.Second,
		Delay:   time.Second,
	})
	return &Engine{
		sched:       NewDefaultScheduler(),
		downloader:  downloader,
		concurrency: 1,
		lg:          logger.NewSugaredLogger("engine", "info"),
		pipelines:   make(map[string][]Pipeline),
	}
}

// UseScheduler sets scheduler
func (e *Engine) UseScheduler(sched Scheduler) {
	e.sched = sched
}

// UseDownloader sets downloader
func (e *Engine) UseDownloader(down Downloader) {
	e.downloader = down
}

// UseLogger sets logger
func (e *Engine) UseLogger(lg logger.Logger) {
	e.lg = lg
}

// RegisterSipders add working spiders
func (e *Engine) RegisterSipders(spiders ...Spider) {
	e.mux.Lock()
	defer e.mux.Unlock()

	for index := range spiders {
		s := spiders[index]
		if s == nil {
			continue
		}
		e.lg.Infof("loading spider [%s]", s.Name())
		e.spiders = append(e.spiders, s)
	}
}

// RegisterPipelines register pipelines
func (e *Engine) RegisterPipelines(pipelines ...Pipeline) {
	for _, p := range pipelines {
		// remove duplicated item name
		tmp := map[string]struct{}{}
		for _, item := range p.ItemList() {
			if _, ok := tmp[item]; ok {
				continue
			}
			e.pipelines[item] = append(e.pipelines[item], p)
			tmp[item] = struct{}{}
		}
	}
}

// Start starts engine
func (e *Engine) Start() {
	if e.state == stateRunning {
		e.lg.Info("engine already running")
		return
	}

	e.state = stateRunning

	e.lg.Info("start engine ...")
	e.sched.Start()

	wg := waitgroup.Wrapper{}

	// load first started requests from all spiders
	e.loadStartRequests()

	for i := 0; i < e.concurrency; i++ {
		// start request handler
		wg.RecoverableWrap(e.requestHandler)
	}

	wg.Wrap(e.requestProbe) // start request probe

	wg.Wait()
}

func (e *Engine) loadStartRequests() {
	e.mux.RLock()
	defer e.mux.RLock()

	for index := range e.spiders {
		spider := e.spiders[index]
		requests := spider.StartRequests()
		for index := range requests {
			e.lg.Infof("adding started reqeust from %s : %s", spider.Name(), requests[index].URL)
			ok := e.sched.AddRequest(requests[index])
			if !ok {
				return
			}
		}
	}
}

func (e *Engine) requestHandler() {
	for {
		req, ok := e.getNextRequest()
		if !ok {
			return
		}

		spiders := e.getRelativeSpider(req.URL)
		if len(spiders) <= 0 {
			e.lg.Warnf("no spider found to handle request: %s", req.URL)
			continue
		}

		resp, err := e.downloader.Download(req)
		if err != nil {
			e.lg.Errorf("<%s %s>  %v", req.Method, req.URL, err)
			continue
		}
		e.lg.Infof("<%s %s %s>", req.Method, req.URL, resp.Status)

		wg := waitgroup.Wrapper{}
		for index := range spiders {
			spider := spiders[index]
			wg.RecoverableWrap(func() {
				e.handleResponse(spider, resp)
			})
		}

		wg.Wait()

		time.Sleep(time.Second)
	}
}

func (e *Engine) getRelativeSpider(url string) []Spider {
	e.mux.RLock()
	defer e.mux.RLocker()

	var spiders []Spider
	for index := range e.spiders {
		spider := e.spiders[index]
		if spider.URLMatcher().Match(url) {
			spiders = append(spiders, spider)
		}
	}
	return spiders
}

// get next request from scheduler.
func (e *Engine) getNextRequest() (*Request, bool) {
	var req *Request
	var hasMore bool
	atomic.AddInt32(&e.pendingCnt, 1)

	for req == nil {
		req, hasMore = e.sched.NextRequest()
		if !hasMore {
			return nil, false
		}
	}

	if req.Method == "" {
		req.Method = http.MethodGet
	}

	atomic.AddInt32(&e.pendingCnt, -1)

	return req, true
}

func (e *Engine) addRequests(reqs []*Request) {
	for index := range reqs {
		req := reqs[index]
		if req == nil {
			continue
		}

		// TODO: limit request depth
		// req.currentDepth = resp.request.currentDepth + 1
		// if req.currentDepth >= 5 {
		// 	continue
		// }

		e.lg.Infof("adding new request [%s %s]", req.Method, req.URL)
		if ok := e.sched.AddRequest(req); !ok {
			return
		}
	}
}

// requestProbe starts a loop to detect whether there is more unhandled request
// in scheduler. Engine will stop if no more requests avaiable.
func (e *Engine) requestProbe() {
	for {
		// if there is no more request in scheduler and the amount of
		// pending workers equals to concurrency, it means all crawling requests
		// has been handled and, probably, there are no more coming requests in the future.
		if !e.sched.HasMore() && (e.pendingCnt == int32(e.concurrency)) {
			// waiting for a while to make sure no more requests
			time.Sleep(time.Millisecond * 500)
			if !e.sched.HasMore() && (e.pendingCnt == int32(e.concurrency)) {
				e.Stop()
				return
			}
		}
		time.Sleep(time.Second)
	}
}

func (e *Engine) handleResponse(spider Spider, resp *Response) {
	ctx := &Context{
		resp: resp,
	}

	items, newReqs, err := spider.Parse(ctx)
	if err != nil {
		e.lg.Errorf("spider [%s] failed to parse result, %v", spider.Name(), err)
		return
	}

	// passing items to all associated pipelines
	e.handleItems(items)

	// TODO:
	// 1 - calculate request depth
	// 2 - FIX IT: create a new goroutine everytime here, may cause too many blocked goroutine
	go e.addRequests(newReqs)
}

func (e *Engine) handleItems(items *Items) {
	if items == nil || items.Name() == "" {
		return
	}

	pipelines, ok := e.pipelines[items.Name()]
	if !ok {
		e.lg.Warnf("no pipeline associate with items: %s", items.Name())
		return
	}

	var wg waitgroup.Wrapper
	for _, p := range pipelines {
		if p == nil {
			continue
		}

		wg.Wrap(func() {
			err := p.Handle(items)
			if err != nil {
				e.lg.Errorf("pipeline [%s] error: %s", p.Name(), err)
			}
		})
	}

	wg.Wait()
}

// Stop stops engine
func (e *Engine) Stop() {
	if e.state == stateStoped {
		return
	}

	e.state = stateStoped
	e.lg.Info("stop engine...")
	e.sched.Stop()
}
