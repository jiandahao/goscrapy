package goscrapy

import "github.com/jiandahao/goutils/channel"

// Scheduler scheduler is responsible for managing all scraping and crawling request
type Scheduler interface {
	Start() error                              // start scheduler
	Stop() error                               // stop scheduler
	NextRequest() (req *Request, hasMore bool) // return next request from scheduler
	AddRequest(req *Request) (ok bool)         // add new request into scheduler
	HasMore() bool                             // returns true if there are more request to be scheduled
}

// DefaultScheduler default scheduler implementation
type DefaultScheduler struct {
	queue channel.Channel
}

// NewDefaultScheduler create a new defult scheduler with queue size of 100
func NewDefaultScheduler() *DefaultScheduler {
	return &DefaultScheduler{
		queue: channel.NewSafeChannel(100),
	}
}

// Start starts scheduler
func (ds *DefaultScheduler) Start() error {
	return nil
}

// Stop stops scheduler
func (ds *DefaultScheduler) Stop() error {
	ds.queue.Close()
	return nil
}

// AddRequest add request
func (ds *DefaultScheduler) AddRequest(req *Request) (ok bool) {
	return ds.queue.Push(req)
}

// NextRequest returns next request
func (ds *DefaultScheduler) NextRequest() (req *Request, hadMore bool) {
	val, ok := ds.queue.Pop()
	if !ok {
		return nil, false
	}

	return val.(*Request), true
}

// HasMore returns true if there are more request to be scheduled
func (ds *DefaultScheduler) HasMore() bool {
	return ds.queue.Count() > 0
}
