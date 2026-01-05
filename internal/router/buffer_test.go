package router

import (
	"sync"
	"testing"
	"time"
)

func TestGrowableBuffer_BasicSendReceive(t *testing.T) {
	buf := NewGrowableBuffer[int](10)

	// Send some items
	for i := 0; i < 5; i++ {
		if !buf.Send(i) {
			t.Fatalf("Send(%d) returned false", i)
		}
	}

	if buf.Len() != 5 {
		t.Errorf("Len() = %d, want 5", buf.Len())
	}

	// Receive items
	for i := 0; i < 5; i++ {
		val, ok := buf.TryReceive()
		if !ok {
			t.Fatalf("TryReceive() returned false for item %d", i)
		}
		if val != i {
			t.Errorf("received %d, want %d", val, i)
		}
	}

	if buf.Len() != 0 {
		t.Errorf("Len() = %d, want 0", buf.Len())
	}
}

func TestGrowableBuffer_GrowAt70Percent(t *testing.T) {
	buf := NewGrowableBuffer[int](10)

	// Send 7 items (70% of 10)
	for i := 0; i < 7; i++ {
		buf.Send(i)
	}

	stats := buf.Stats()
	if stats.Capacity <= 10 {
		t.Errorf("Capacity = %d, expected growth after 70%% fill", stats.Capacity)
	}
	if stats.ResizeCount != 1 {
		t.Errorf("ResizeCount = %d, want 1", stats.ResizeCount)
	}

	// All items should still be accessible
	for i := 0; i < 7; i++ {
		val, ok := buf.TryReceive()
		if !ok {
			t.Fatalf("TryReceive() returned false for item %d", i)
		}
		if val != i {
			t.Errorf("received %d, want %d", val, i)
		}
	}
}

func TestGrowableBuffer_MultipleGrows(t *testing.T) {
	buf := NewGrowableBuffer[int](4)

	// Send 100 items - should trigger multiple grows
	for i := 0; i < 100; i++ {
		if !buf.Send(i) {
			t.Fatalf("Send(%d) returned false", i)
		}
	}

	stats := buf.Stats()
	if stats.Count != 100 {
		t.Errorf("Count = %d, want 100", stats.Count)
	}
	if stats.ResizeCount < 3 {
		t.Errorf("ResizeCount = %d, expected at least 3 resizes", stats.ResizeCount)
	}

	// Verify all items in order
	for i := 0; i < 100; i++ {
		val, ok := buf.TryReceive()
		if !ok {
			t.Fatalf("TryReceive() returned false for item %d", i)
		}
		if val != i {
			t.Errorf("received %d, want %d", val, i)
		}
	}
}

func TestGrowableBuffer_BlockingReceive(t *testing.T) {
	buf := NewGrowableBuffer[int](10)

	received := make(chan int, 1)

	// Start goroutine that waits for data
	go func() {
		val, ok := buf.Receive()
		if ok {
			received <- val
		}
	}()

	// Give receiver time to start waiting
	time.Sleep(10 * time.Millisecond)

	// Send data
	buf.Send(42)

	// Should receive the value
	select {
	case val := <-received:
		if val != 42 {
			t.Errorf("received %d, want 42", val)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for blocked receive")
	}
}

func TestGrowableBuffer_Close(t *testing.T) {
	buf := NewGrowableBuffer[int](10)

	// Send some items
	buf.Send(1)
	buf.Send(2)

	// Close
	buf.Close()

	// Send should return false after close
	if buf.Send(3) {
		t.Error("Send should return false after Close")
	}

	// Can still receive existing items
	val, ok := buf.TryReceive()
	if !ok || val != 1 {
		t.Errorf("TryReceive() = %d, %v; want 1, true", val, ok)
	}

	val, ok = buf.TryReceive()
	if !ok || val != 2 {
		t.Errorf("TryReceive() = %d, %v; want 2, true", val, ok)
	}

	// No more items
	_, ok = buf.TryReceive()
	if ok {
		t.Error("TryReceive should return false when empty and closed")
	}
}

func TestGrowableBuffer_CloseUnblocksReceive(t *testing.T) {
	buf := NewGrowableBuffer[int](10)

	done := make(chan bool, 1)

	go func() {
		_, ok := buf.Receive()
		done <- ok
	}()

	// Give receiver time to start waiting
	time.Sleep(10 * time.Millisecond)

	// Close should unblock the receiver
	buf.Close()

	select {
	case ok := <-done:
		if ok {
			t.Error("Receive should return false when closed and empty")
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not unblock Receive")
	}
}

func TestGrowableBuffer_DrainTo(t *testing.T) {
	buf := NewGrowableBuffer[int](10)

	// Send 10 items
	for i := 0; i < 10; i++ {
		buf.Send(i)
	}

	// Drain 5 items
	items := buf.DrainTo(5)
	if len(items) != 5 {
		t.Errorf("DrainTo(5) returned %d items, want 5", len(items))
	}
	for i, val := range items {
		if val != i {
			t.Errorf("items[%d] = %d, want %d", i, val, i)
		}
	}

	// 5 items should remain
	if buf.Len() != 5 {
		t.Errorf("Len() = %d, want 5", buf.Len())
	}

	// Drain all remaining
	items = buf.DrainTo(0) // 0 means all
	if len(items) != 5 {
		t.Errorf("DrainTo(0) returned %d items, want 5", len(items))
	}

	if buf.Len() != 0 {
		t.Errorf("Len() = %d, want 0", buf.Len())
	}
}

func TestGrowableBuffer_ConcurrentSendReceive(t *testing.T) {
	buf := NewGrowableBuffer[int](10)
	const numItems = 1000

	var wg sync.WaitGroup

	// Sender
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numItems; i++ {
			buf.Send(i)
		}
	}()

	// Receiver
	received := make([]int, 0, numItems)
	var mu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numItems; i++ {
			val, ok := buf.Receive()
			if ok {
				mu.Lock()
				received = append(received, val)
				mu.Unlock()
			}
		}
	}()

	wg.Wait()

	if len(received) != numItems {
		t.Errorf("received %d items, want %d", len(received), numItems)
	}

	// Verify we got all items (order may vary due to concurrency)
	seen := make(map[int]bool)
	for _, val := range received {
		seen[val] = true
	}
	for i := 0; i < numItems; i++ {
		if !seen[i] {
			t.Errorf("missing item %d", i)
		}
	}
}

func TestGrowableBuffer_WrapAround(t *testing.T) {
	buf := NewGrowableBuffer[int](5)

	// Fill partially
	buf.Send(1)
	buf.Send(2)
	buf.Send(3)

	// Consume some
	buf.TryReceive() // removes 1
	buf.TryReceive() // removes 2

	// Add more - this wraps around
	buf.Send(4)
	buf.Send(5)
	buf.Send(6)

	// Now trigger growth with wrap-around
	buf.Send(7)
	buf.Send(8)

	// Verify all items in order
	expected := []int{3, 4, 5, 6, 7, 8}
	for _, want := range expected {
		got, ok := buf.TryReceive()
		if !ok {
			t.Fatalf("TryReceive failed, expected %d", want)
		}
		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	}
}

func TestGrowableBuffer_Stats(t *testing.T) {
	buf := NewGrowableBuffer[int](10)

	// Initial stats
	stats := buf.Stats()
	if stats.Count != 0 || stats.Capacity != 10 || stats.TotalReceived != 0 || stats.TotalSent != 0 {
		t.Errorf("initial stats incorrect: %+v", stats)
	}

	// After sends
	buf.Send(1)
	buf.Send(2)
	buf.Send(3)

	stats = buf.Stats()
	if stats.Count != 3 || stats.TotalReceived != 3 {
		t.Errorf("stats after sends: %+v", stats)
	}

	// After receives
	buf.TryReceive()
	buf.TryReceive()

	stats = buf.Stats()
	if stats.Count != 1 || stats.TotalSent != 2 {
		t.Errorf("stats after receives: %+v", stats)
	}
}

func TestNewGrowableBuffer_MinCapacity(t *testing.T) {
	// Capacity of 0 should be set to 1
	buf := NewGrowableBuffer[int](0)
	if buf.Cap() != 1 {
		t.Errorf("Cap() = %d, want 1 for initial capacity 0", buf.Cap())
	}

	// Negative capacity should be set to 1
	buf = NewGrowableBuffer[int](-5)
	if buf.Cap() != 1 {
		t.Errorf("Cap() = %d, want 1 for negative initial capacity", buf.Cap())
	}
}
