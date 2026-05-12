package kube

import (
	"sync"

	"k8s.io/client-go/tools/remotecommand"
)

type TerminalResizeQueue struct {
	ch     chan remotecommand.TerminalSize
	mu     sync.RWMutex
	closed bool
	once   sync.Once
}

func NewTerminalResizeQueue() *TerminalResizeQueue {
	return &TerminalResizeQueue{ch: make(chan remotecommand.TerminalSize, 4)}
}

func (q *TerminalResizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-q.ch
	if !ok {
		return nil
	}
	return &size
}

func (q *TerminalResizeQueue) Push(width, height uint16) {
	if q == nil || width == 0 || height == 0 {
		return
	}
	size := remotecommand.TerminalSize{Width: width, Height: height}
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.closed {
		return
	}
	select {
	case q.ch <- size:
	default:
		select {
		case <-q.ch:
		default:
		}
		q.ch <- size
	}
}

func (q *TerminalResizeQueue) Close() {
	if q == nil {
		return
	}
	q.once.Do(func() {
		q.mu.Lock()
		defer q.mu.Unlock()
		q.closed = true
		close(q.ch)
	})
}
