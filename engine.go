package goscrapy

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	uuid "github.com/hashicorp/go-uuid"
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

	requestHandlers  []RequestHandleFunc
	responseHandlers []ResponseHandleFunc
	maxCrawlingDepth int // max crawling depth, no limit if less or equals to 0
}

// New create a new goscrapy engine
func New(opts ...Option) *Engine {
	e := &Engine{
		sched:       NewFIFOScheduler(), // using a fifo scheduler by default
		downloader:  &DefaultDownloader{},
		concurrency: 1,
		lg:          logger.NewDefaultLogger("info"),
		pipelines:   make(map[string][]Pipeline),
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Option engine option
type Option func(e *Engine)

// SetConcurrency set concurrency
func SetConcurrency(num int) Option {
	return func(e *Engine) {
		if num <= 0 {
			return
		}
		e.concurrency = num
	}
}

// UseLogger set logger
func UseLogger(lg logger.Logger) Option {
	return func(e *Engine) {
		e.lg = lg
	}
}

// UseDownloader set downloader
func UseDownloader(d Downloader) Option {
	return func(e *Engine) {
		e.downloader = d
	}
}

// UseScheduler set scheduler
func UseScheduler(s Scheduler) Option {
	return func(e *Engine) {
		e.sched = s
	}
}

// WithRequestMiddlewares registers request middlewares. Requests will be processed
// by request middlewares just before passing to downloader.
//
// for aborting request in middleware, using Request.Abrot()
/* for example:
func ReqMiddleware(req *goscrapy.Request) error {
	if req.URL == "http://www.example.com" {
		req.Abort()
		return nil
	}

	return nil
}
*/
func WithRequestMiddlewares(middlewares ...RequestHandleFunc) Option {
	return func(e *Engine) {
		e.requestHandlers = append(e.requestHandlers, middlewares...)
	}
}

// WithResponseMiddlewares registers response middlewares. Response will be processed by
// response middlewares right after downloader finishes downloading and takes over
// response to engine
func WithResponseMiddlewares(middlewares ...ResponseHandleFunc) Option {
	return func(e *Engine) {
		e.responseHandlers = append(e.responseHandlers, middlewares...)
	}
}

// MaxCrawlingDepth returns an Option that sets the max crawling depth. The engine will drop
// Requests that have current depth exceeded the maximum limit.
func MaxCrawlingDepth(depth int) Option {
	return func(e *Engine) {
		e.maxCrawlingDepth = depth
	}
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
		e.lg.Infof(context.Background(), "loading spider [%s]", s.Name())
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
	ctx := context.Background()
	if e.state == stateRunning {
		e.lg.Infof(ctx, "engine already running")
		return
	}

	e.state = stateRunning

	e.lg.Infof(ctx, "start engine ...")
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
			e.lg.Infof(context.Background(), "adding started reqeust from %s : %s", spider.Name(), requests[index].URL)
			req := requests[index]
			req.currentDepth = 1
			ok := e.sched.PushRequest(req)
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

		requestID, _ := uuid.GenerateUUID()
		ctx := logger.AppendMetadata(
			context.Background(),
			logger.NewMetadata().Append("request_id", requestID),
		)

		spiders := e.getRelativeSpider(req.URL)
		if len(spiders) <= 0 {
			e.lg.Warnf(ctx, "no spider found to handle request: %s", req.URL)
			continue
		}

		resp, err := e.handleRequest(ctx, req)
		if err != nil {
			e.lg.Errorf(ctx, "<%s %s>  %v", req.Method, req.URL, err)
			continue
		}

		if resp == nil {
			continue
		}

		e.lg.Infof(ctx, "<%s %s %s>", req.Method, req.URL, resp.Status)

		e.handleResponse(ctx, spiders, resp)

		time.Sleep(10 * time.Second)
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
		req, hasMore = e.sched.PopRequest()
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

func (e *Engine) addRequests(ctx *Context, reqs []*Request) {
	for index := range reqs {
		req := reqs[index]
		if req == nil {
			continue
		}

		req.currentDepth = ctx.Request().currentDepth + 1
		if e.maxCrawlingDepth > 0 && req.currentDepth > e.maxCrawlingDepth {
			// has exceeds max crawling depth, drop it !!!
			e.lg.Debugf(ctx, "exceeds max crawling depth [max=%v], drop request: %s", e.maxCrawlingDepth, req.URL)
			continue
		}

		e.lg.Infof(ctx, "adding new request [%s %s]", req.Method, req.URL)
		if ok := e.sched.PushRequest(req); !ok {
			return
		}
	}
}

// requestProbe starts a loop to detect whether there is more unhandled request
// in scheduler. Engine will stop if no more requests available.
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

func (e *Engine) handleRequest(ctx context.Context, req *Request) (*Response, error) {
	// handle request using middlewares before passing to downloader
	for _, fn := range e.requestHandlers {
		err := fn(req)
		if err != nil {
			return nil, err
		}

		if req.IsAborted() {
			return nil, nil
		}
	}

	return e.downloader.Download(req)
}

func (e *Engine) handleResponse(ctx context.Context, spiders []Spider, resp *Response) {
	// handle response using middleware before passing to spider
	for _, fn := range e.responseHandlers {
		err := fn(resp)
		if err != nil {
			e.lg.Errorf(ctx, "handle response failure in middleware, %v", err)
			return
		}
	}

	wg := waitgroup.Wrapper{}
	for index := range spiders {
		spider := spiders[index]
		wg.RecoverableWrap(func() {
			sctx := &Context{
				Context:  ctx,
				response: resp,
			}

			items, newReqs, err := spider.Parse(sctx)
			if err != nil {
				e.lg.Errorf(ctx, "spider [%s] failed to parse result, %v", spider.Name(), err)
				return
			}

			// passing items to all associated pipelines
			e.handleItems(sctx, items)

			// TODO:
			// 1 - calculate request depth
			// 2 - FIX IT: create a new goroutine everytime here, may cause too many blocked goroutine
			go e.addRequests(sctx, newReqs)
		})
	}
	wg.Wait()
}

func (e *Engine) handleItems(ctx context.Context, items *Items) {
	if items == nil || items.Name() == "" {
		return
	}

	pipelines, ok := e.pipelines[items.Name()]
	if !ok {
		e.lg.Warnf(ctx, "no pipeline associate with items: %s", items.Name())
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
				e.lg.Errorf(ctx, "pipeline [%s] error: %s", p.Name(), err)
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
	e.lg.Infof(context.Background(), "stop engine...")
	e.sched.Stop()
}
