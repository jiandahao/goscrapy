package goscrapy

import (
	"container/heap"
	"sync"

	"github.com/jiandahao/goutils/channel"
)

// Scheduler scheduler is responsible for managing all scraping and crawling request
type Scheduler interface {
	Start() error                              // start scheduler
	Stop() error                               // stop scheduler
	NextRequest() (req *Request, hasMore bool) // return next request from scheduler
	AddRequest(req *Request) (ok bool)         // add new request into scheduler
	HasMore() bool                             // returns true if there are more request to be scheduled
}

// FIFOScheduler default scheduler implementation
type FIFOScheduler struct {
	queue channel.Channel
}

// NewFIFOScheduler create a new fifo scheduler with queue size of 100
func NewFIFOScheduler() *FIFOScheduler {
	return &FIFOScheduler{
		queue: channel.NewSafeChannel(100),
	}
}

// Start starts scheduler
func (ds *FIFOScheduler) Start() error {
	return nil
}

// Stop stops scheduler
func (ds *FIFOScheduler) Stop() error {
	ds.queue.Close()
	return nil
}

// AddRequest add request
func (ds *FIFOScheduler) AddRequest(req *Request) (ok bool) {
	return ds.queue.Push(req)
}

// NextRequest returns next request
func (ds *FIFOScheduler) NextRequest() (req *Request, hadMore bool) {
	val, ok := ds.queue.Pop()
	if !ok {
		return nil, false
	}

	return val.(*Request), true
}

// HasMore returns true if there are more request to be scheduled
func (ds *FIFOScheduler) HasMore() bool {
	return ds.queue.Count() > 0
}

// WeightedScheduler scheduler
type WeightedScheduler struct {
	data []Request
	mux  sync.RWMutex
	c    chan Request
}

// NewWeightedScheduler new a weighted scheduler which is implemented
// based on max-heap.
func NewWeightedScheduler() *WeightedScheduler {
	return &WeightedScheduler{
		c: make(chan Request, 500),
	}
}

// Start start
func (sched *WeightedScheduler) Start() error {
	go func() {
		for req := range sched.c {
			sched.mux.Lock()
			heap.Push(sched, req)
			sched.mux.Unlock()
		}
	}()
	return nil
}

// Stop stop
func (sched *WeightedScheduler) Stop() error {
	return nil
}

// PushRequest push request
func (sched *WeightedScheduler) PushRequest(req Request) (ok bool) {
	sched.c <- req
	return true
}

// PopRequest pop request
func (sched *WeightedScheduler) PopRequest() (req *Request, ok bool) {
	sched.mux.Lock()
	defer sched.mux.Unlock()

	if sched.Len() <= 0 {
		return nil, true
	}

	val := heap.Pop(sched)
	res, ok := val.(Request)
	return &res, true
}

// HasMore returns true if queue has more request
func (sched *WeightedScheduler) HasMore() bool {
	sched.mux.RLock()
	defer sched.mux.RUnlock()

	return sched.Len() > 0
}

// Len returns the number of elements in the collection.
func (sched *WeightedScheduler) Len() int {
	return len(sched.data)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (sched *WeightedScheduler) Less(i, j int) bool {
	return sched.data[i].Weight > sched.data[j].Weight
}

// Swap swaps the elements with indexes i and j.
func (sched *WeightedScheduler) Swap(i, j int) {
	sched.data[i], sched.data[j] = sched.data[j], sched.data[i]
}

// Push pushes value onto heap, it's aimed to implement sort.Interface.
// For pushing request onto schduler using PushRequest instead.
func (sched *WeightedScheduler) Push(x interface{}) {
	sched.data = append(sched.data, x.(Request))
}

// Pop remove and return element Len() - 1. It's aimed to implement sort.Interface.
// For pushing request onto schduler using PopRequest instead.
func (sched *WeightedScheduler) Pop() interface{} {
	if sched.Len() <= 0 {
		return nil
	}

	res := sched.data[sched.Len()-1]
	sched.data = sched.data[:sched.Len()-1]

	return res
}
