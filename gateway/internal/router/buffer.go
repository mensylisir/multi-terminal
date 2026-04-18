package router

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

const (
	BufferSize    = 1024 * 1024 // 1MB
	WaterMarkHigh = BufferSize * 85 / 100
	WaterMarkLow  = BufferSize * 50 / 100
)

var (
	ErrBufferFull  = errors.New("buffer is full")
	ErrBufferEmpty = errors.New("buffer is empty")
)

type RingBuffer struct {
	data   []byte
	write  atomic.Int64
	read   atomic.Int64
	mu     sync.Mutex
	cond   *sync.Cond
	closed bool
}

func NewRingBuffer() *RingBuffer {
	rb := &RingBuffer{
		data: make([]byte, BufferSize),
	}
	rb.cond = sync.NewCond(&rb.mu)
	return rb
}

func (rb *RingBuffer) Write(p []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return 0, io.EOF
	}

	available := BufferSize - int(rb.write.Load()-rb.read.Load())
	if available < len(p) {
		// Buffer full, write what we can
		n := copy(rb.data[rb.write.Load()%BufferSize:], p)
		rb.write.Add(int64(n))
		rb.cond.Signal()
		return n, ErrBufferFull
	}

	n := copy(rb.data[rb.write.Load()%BufferSize:], p)
	rb.write.Add(int64(n))
	rb.cond.Signal()
	return n, nil
}

func (rb *RingBuffer) Read(p []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for rb.read.Load() >= rb.write.Load() && !rb.closed {
		rb.cond.Wait()
	}

	if rb.read.Load() >= rb.write.Load() && rb.closed {
		return 0, io.EOF
	}

	n := copy(p, rb.data[rb.read.Load()%BufferSize:])
	rb.read.Add(int64(n))
	return n, nil
}

func (rb *RingBuffer) Available() int {
	return int(rb.write.Load() - rb.read.Load())
}

func (rb *RingBuffer) IsHigh() bool {
	return rb.Available() >= WaterMarkHigh
}

func (rb *RingBuffer) IsLow() bool {
	return rb.Available() <= WaterMarkLow
}

func (rb *RingBuffer) Close() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.closed = true
	rb.cond.Broadcast()
}