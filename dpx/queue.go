package dpx

import (
	"errors"
	"sync"

	"github.com/hishboy/gocommons/lang"
)

// Here is a shitty queue with blocking capabilities.
// Replace me

type queue struct {
	sync.Mutex
	Q        *lang.Queue
	ch       chan struct{}
	draining bool
	closed   bool
	err      error
}

func newQueue() *queue {
	q := &queue{Q: lang.NewQueue()}
	q.ch = make(chan struct{}, 1024)
	return q
}

func (q *queue) Enqueue(item interface{}) error {
	q.Lock()
	defer q.Unlock()
	if q.err != nil {
		return q.err
	}
	q.Q.Push(item)
	q.ch <- struct{}{}
	return nil
}

func (q *queue) EnqueueLast(item interface{}) (err error) {
	q.Lock()
	defer q.Unlock()
	if q.err != nil {
		return q.err
	}
	q.Q.Push(item)
	q.draining = true
	q.err = errors.New("queue already received last")
	q.ch <- struct{}{}
	return nil
}

func (q *queue) Close() {
	q.Lock()
	defer q.Unlock()
	q.closed = true
	q.err = errors.New("queue has been closed")
	q.Q = nil
}

func (q *queue) Closed() bool {
	return q.closed
}

func (q *queue) Draining() bool {
	return q.draining
}

func (q *queue) Dequeue() interface{} {
	if q.closed {
		return nil
	}
	<-q.ch
	return q.Q.Poll()
}
