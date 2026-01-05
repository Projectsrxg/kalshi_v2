package router

import (
	"sync"
)

// GrowableBuffer is a thread-safe buffer that automatically doubles
// its capacity when it reaches 70% full.
type GrowableBuffer[T any] struct {
	mu       sync.Mutex
	cond     *sync.Cond
	buf      []T
	head     int // read position
	tail     int // write position
	count    int
	capacity int
	closed   bool

	// Stats
	totalReceived int64
	totalSent     int64
	resizeCount   int
}

// NewGrowableBuffer creates a new buffer with the given initial capacity.
func NewGrowableBuffer[T any](initialCapacity int) *GrowableBuffer[T] {
	if initialCapacity < 1 {
		initialCapacity = 1
	}
	b := &GrowableBuffer[T]{
		buf:      make([]T, initialCapacity),
		capacity: initialCapacity,
	}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Send adds an item to the buffer. Grows the buffer if at 70% capacity.
// Returns false if the buffer is closed.
func (b *GrowableBuffer[T]) Send(item T) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return false
	}

	// Check if we need to grow (at or above 70% capacity after adding this item)
	threshold := (b.capacity * 70) / 100
	if threshold < 1 {
		threshold = 1
	}
	if b.count+1 >= threshold {
		b.grow()
	}

	// Add item
	b.buf[b.tail] = item
	b.tail = (b.tail + 1) % b.capacity
	b.count++
	b.totalReceived++

	// Signal waiting receivers
	b.cond.Signal()
	return true
}

// Receive removes and returns an item from the buffer.
// Blocks until an item is available or the buffer is closed.
// Returns the item and true, or zero value and false if closed and empty.
func (b *GrowableBuffer[T]) Receive() (T, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Wait for data or close
	for b.count == 0 && !b.closed {
		b.cond.Wait()
	}

	if b.count == 0 && b.closed {
		var zero T
		return zero, false
	}

	item := b.buf[b.head]
	var zero T
	b.buf[b.head] = zero // Clear reference for GC
	b.head = (b.head + 1) % b.capacity
	b.count--
	b.totalSent++

	return item, true
}

// TryReceive attempts to receive without blocking.
// Returns the item and true if available, or zero value and false otherwise.
func (b *GrowableBuffer[T]) TryReceive() (T, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.count == 0 {
		var zero T
		return zero, false
	}

	item := b.buf[b.head]
	var zero T
	b.buf[b.head] = zero
	b.head = (b.head + 1) % b.capacity
	b.count--
	b.totalSent++

	return item, true
}

// Close closes the buffer. After closing, Send returns false.
// Receivers will get remaining items then receive closed signal.
func (b *GrowableBuffer[T]) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	b.cond.Broadcast() // Wake all waiters
}

// Len returns the current number of items in the buffer.
func (b *GrowableBuffer[T]) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

// Cap returns the current capacity of the buffer.
func (b *GrowableBuffer[T]) Cap() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.capacity
}

// Stats returns buffer statistics.
func (b *GrowableBuffer[T]) Stats() BufferStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return BufferStats{
		Count:         b.count,
		Capacity:      b.capacity,
		TotalReceived: b.totalReceived,
		TotalSent:     b.totalSent,
		ResizeCount:   b.resizeCount,
	}
}

// BufferStats contains buffer statistics.
type BufferStats struct {
	Count         int
	Capacity      int
	TotalReceived int64
	TotalSent     int64
	ResizeCount   int
}

// grow doubles the buffer capacity. Must be called with lock held.
func (b *GrowableBuffer[T]) grow() {
	newCapacity := b.capacity * 2
	newBuf := make([]T, newCapacity)

	// Copy existing items to new buffer
	if b.count > 0 {
		if b.head < b.tail {
			// Contiguous: [head...tail)
			copy(newBuf, b.buf[b.head:b.tail])
		} else {
			// Wrapped: [head...end) + [0...tail)
			n := copy(newBuf, b.buf[b.head:])
			copy(newBuf[n:], b.buf[:b.tail])
		}
	}

	b.buf = newBuf
	b.head = 0
	b.tail = b.count
	b.capacity = newCapacity
	b.resizeCount++
}

// DrainTo drains all items from the buffer into the provided slice.
// Returns the items drained. Useful for batch processing.
func (b *GrowableBuffer[T]) DrainTo(max int) []T {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.count == 0 {
		return nil
	}

	n := b.count
	if max > 0 && max < n {
		n = max
	}

	result := make([]T, n)
	for i := 0; i < n; i++ {
		result[i] = b.buf[b.head]
		var zero T
		b.buf[b.head] = zero
		b.head = (b.head + 1) % b.capacity
		b.count--
		b.totalSent++
	}

	return result
}
