package main

import (
	log "code.google.com/p/log4go"
	"errors"
	"net"
	"sync"
	"time"
)

const (
	timerDelay = 100 * time.Millisecond
)

var (
	ErrTimerFull   = errors.New("timer full")
	ErrTimerEmpty  = errors.New("timer empty")
	ErrTimerNoItem = errors.New("timer item not exist")
)

type TimerData struct {
	Key   int64
	Value net.Conn
	index int
}

type Timer struct {
	cur    int
	max    int
	lock   sync.Mutex
	timers []*TimerData
}

// A heap must be initialized before any of the heap operations
// can be used. Init is idempotent with respect to the heap invariants
// and may be called whenever the heap invariants may have been invalidated.
// Its complexity is O(n) where n = h.Len().
//
func NewTimer(num int) *Timer {
	// heapify
	t := new(Timer)
	t.timers = make([]*TimerData, num, num)
	t.cur = -1
	t.max = num - 1
	/*
		n := h.Len()
		for i := n/2 - 1; i >= 0; i-- {
			t.down(i, n)
		}
	*/
	go t.process()
	return t
}

// Push pushes the element x onto the heap. The complexity is
// O(log(n)) where n = h.Len().
//
func (t *Timer) Push(item *TimerData) error {
	log.Debug("timer: before push cur: %d, max: %d", t.cur, t.max)
	t.lock.Lock()
	if t.cur >= t.max {
		t.lock.Unlock()
		return ErrTimerFull
	}
	t.cur++
	item.index = t.cur
	// add to the minheap last node
	t.timers[t.cur] = item
	t.up(t.cur - 1)
	t.lock.Unlock()
	log.Debug("timer: after push cur: %d, max: %d", t.cur, t.max)
	return nil
}

// Pop removes the minimum element (according to Less) from the heap
// and returns it. The complexity is O(log(n)) where n = h.Len().
// It is equivalent to Remove(h, 0).
//
func (t *Timer) Pop() (item *TimerData, err error) {
	log.Debug("timer: before pop cur: %d, max: %d", t.cur, t.max)
	t.lock.Lock()
	if t.cur < 0 {
		err = ErrTimerEmpty
		return
	}
	t.swap(0, t.cur)
	t.down(0, t.cur)
	// remove last element
	item = t.pop()
	t.lock.Unlock()
	log.Debug("timer: after pop cur: %d, max: %d", t.cur, t.max)
	return
}

// Remove removes the element at index i from the heap.
// The complexity is O(log(n)) where n = h.Len().
//
func (t *Timer) Remove(item *TimerData) (nitem *TimerData, err error) {
	log.Debug("timer: remove item Key: %d", item.Key)
	t.lock.Lock()
	if item.index == -1 {
		err = ErrTimerNoItem
		return
	}
	if t.cur != item.index {
		// swap the last node
		// let the big one down
		// let the small one up
		t.swap(item.index, t.cur)
		t.down(item.index, t.cur)
		t.up(item.index)
	}
	// remove item is the last node
	nitem = t.pop()
	t.lock.Unlock()
	log.Debug("timer: before remove cur: %d, max: %d", t.cur, t.max)
	return
}

func (t *Timer) Peek() (item *TimerData, err error) {
	if t.cur < 0 {
		err = ErrTimerEmpty
		return
	}
	item = t.timers[0]
	return
}

func (t *Timer) up(j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || !t.less(j, i) {
			break
		}
		t.swap(i, j)
		j = i
	}
}

func (t *Timer) down(i, n int) {
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && !t.less(j1, j2) {
			j = j2 // = 2*i + 2  // right child
		}
		if !t.less(j, i) {
			break
		}
		t.swap(i, j)
		i = j
	}
}

func (t *Timer) less(i, j int) bool {
	return t.timers[i].Key < t.timers[j].Key
}

func (t *Timer) swap(i, j int) {
	t.timers[i], t.timers[j] = t.timers[j], t.timers[i]
	t.timers[i].index = i
	t.timers[j].index = j
}

func (t *Timer) pop() (item *TimerData) {
	item = t.timers[t.cur]
	item.index = -1 // for safety
	t.cur--
	return
}

func (t *Timer) process() {
	var (
		err   error
		td    *TimerData
		now   int64
		sleep int64
	)
	log.Info("start process timer")
	for {
		if td, err = t.Peek(); err != nil {
			log.Debug("timer: no expire")
			time.Sleep(timerDelay)
			continue
		}
		now = time.Now().UnixNano()
		if sleep = (td.Key - now); sleep > 0 {
			log.Debug("timer: delay %d millisecond", sleep*int64(time.Nanosecond)/int64(time.Millisecond))
			time.Sleep(time.Duration(sleep))
			continue
		}
		if td, err = t.Pop(); err != nil {
			time.Sleep(timerDelay)
			continue
		}
		log.Debug("expire timer: %d", td.Key)
		if td.Value == nil {
			log.Warn("expire timer no net.Conn")
			continue
		}
		if err = td.Value.Close(); err != nil {
			log.Error("timer conn close error(%v)", err)
		}
	}
}
