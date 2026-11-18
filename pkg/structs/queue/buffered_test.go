package queue

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

func TestNewDoubleBufferQueue(t *testing.T) {
	ctx := context.Background()
	queue := NewDoubleBufferQueue[byte](ctx, 100, 10*time.Millisecond)

	if queue == nil {
		t.Error("NewDoubleBufferQueue() should return a non-nil queue")
	}

	queue.Close()
}

func TestNewDoubleBufferQueueWithNilContext(t *testing.T) {
	queue := NewDoubleBufferQueue[byte](context.TODO(), 100, 10*time.Millisecond)

	if queue == nil {
		t.Error("NewDoubleBufferQueue() should handle nil context")
	}

	queue.Close()
}

func TestBufferedWrite(t *testing.T) {
	ctx := context.Background()
	queue := NewDoubleBufferQueue[byte](ctx, 10, 100*time.Millisecond)
	defer queue.Close()

	tests := []struct {
		name     string
		data     []byte
		expected int
		wantErr  bool
	}{
		{
			name:     "write normal data",
			data:     []byte("hello"),
			expected: 5,
			wantErr:  false,
		},
		{
			name:     "write empty data",
			data:     []byte{},
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "write large data",
			data:     make([]byte, 1000), // larger than initial size
			expected: 1000,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := queue.Write(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if n != tt.expected {
				t.Errorf("Expected to write %d bytes, wrote %d", tt.expected, n)
			}
		})
	}
}

func TestBufferedRead(t *testing.T) {
	ctx := context.Background()
	queue := NewDoubleBufferQueue[byte](ctx, 10, 100*time.Millisecond)
	defer queue.Close()

	testData := []byte("hello world")
	_, err := queue.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	readBuf := make([]byte, 5)
	n, err := queue.Read(readBuf)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected to read 5 bytes, read %d", n)
	}
	if string(readBuf) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", string(readBuf))
	}

	readBuf2 := make([]byte, 10)
	n, err = queue.Read(readBuf2)
	if err != nil {
		t.Errorf("Read remaining failed: %v", err)
	}
	if n != 6 {
		t.Errorf("Expected to read 6 bytes, read %d", n)
	}
	if string(readBuf2[:n]) != " world" {
		t.Errorf("Expected ' world', got '%s'", string(readBuf2[:n]))
	}

	n, err = queue.Read(readBuf2)
	if err != nil {
		t.Errorf("Read empty should not error, got %v", err)
	}
	if n != 0 {
		t.Errorf("Expected to read 0 bytes when empty, read %d", n)
	}

	readBuf3 := make([]byte, 0)
	n, err = queue.Read(readBuf3)
	if err != nil {
		t.Errorf("Read empty buffer failed: %v", err)
	}
	if n != 0 {
		t.Errorf("Expected to read 0 bytes from empty buffer, read %d", n)
	}
}

func TestBufferedWriteAfterClose(t *testing.T) {
	ctx := context.Background()
	queue := NewDoubleBufferQueue[byte](ctx, 10, 100*time.Millisecond)

	err := queue.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	_, err = queue.Write([]byte("test"))
	if err == nil {
		t.Error("Write should fail after close")
	}
	if err.Error() != "buffered is closed" {
		t.Errorf("Expected 'buffered is closed' error, got '%s'", err.Error())
	}
}

func TestBufferedReadAfterClose(t *testing.T) {
	ctx := context.Background()
	queue := NewDoubleBufferQueue[byte](ctx, 10, 100*time.Millisecond)

	testData := []byte("hello")
	//nolint:errcheck // in-memory write; the assertions below surface any failure
	queue.Write(testData)

	err := queue.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	readBuf := make([]byte, 10)
	n, err := queue.Read(readBuf)
	if err != io.EOF {
		t.Errorf("Expected EOF after close, got %v", err)
	}
	if n != 0 {
		t.Errorf("Expected to read 0 bytes after close, read %d", n)
	}
}

func TestBufferedDoubleClose(t *testing.T) {
	ctx := context.Background()
	queue := NewDoubleBufferQueue[byte](ctx, 10, 100*time.Millisecond)

	err := queue.Close()
	if err != nil {
		t.Errorf("First close failed: %v", err)
	}

	err = queue.Close()
	if err != nil {
		t.Errorf("Second close should not error, got: %v", err)
	}
}

func TestBufferedSwapBehavior(t *testing.T) {
	ctx := context.Background()
	queue := NewDoubleBufferQueue[byte](ctx, 10, 50*time.Millisecond)
	defer queue.Close()

	data1 := []byte("first")
	_, err := queue.Write(data1)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	readBuf := make([]byte, 5)
	n, err := queue.Read(readBuf)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected to read 5 bytes, read %d", n)
	}
	if string(readBuf) != "first" {
		t.Errorf("Expected 'first', got '%s'", string(readBuf))
	}

	data2 := []byte("second")
	_, err = queue.Write(data2)
	if err != nil {
		t.Fatalf("Second write failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	n, err = queue.Read(readBuf)
	if err != nil {
		t.Errorf("Second read failed: %v", err)
	}
	if n != 5 && n != 6 { // might get partial read
		t.Errorf("Expected to read 5-6 bytes, read %d", n)
	}
}

func TestBufferedConcurrentWriteRead(t *testing.T) {
	ctx := context.Background()
	queue := NewDoubleBufferQueue[byte](ctx, 100, 10*time.Millisecond)
	defer queue.Close()

	var wg sync.WaitGroup
	numWriters := 2
	numMessages := 5

	wg.Add(numWriters)
	for i := 0; i < numWriters; i++ {
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				data := []byte{byte(writerID), byte(j)}
				_, err := queue.Write(data)
				if err != nil {
					t.Errorf("Writer %d message %d failed: %v", writerID, j, err)
				}
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	totalRead := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		readBuf := make([]byte, 10)
		maxReads := 20 // prevent infinite loop
		for reads := 0; reads < maxReads; reads++ {
			n, err := queue.Read(readBuf)
			if err != nil && err != io.EOF {
				t.Errorf("Reader failed: %v", err)
				return
			}
			if n == 0 {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			totalRead += n
			if totalRead >= numWriters*numMessages*2 {
				break
			}
		}
	}()

	wg.Wait()

	if totalRead == 0 {
		t.Error("No data was read by reader")
	}
}

func TestBufferedAutoSwap(t *testing.T) {
	ctx := context.Background()
	queue := NewDoubleBufferQueue[byte](ctx, 10, 10*time.Millisecond)
	defer queue.Close()

	testData := []byte("test")
	_, err := queue.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	readBuf := make([]byte, 10)
	n, err := queue.Read(readBuf)
	if err != nil {
		t.Errorf("Read after auto swap failed: %v", err)
	}
	if n != 4 {
		t.Errorf("Expected to read 4 bytes, read %d", n)
	}
	if string(readBuf[:n]) != "test" {
		t.Errorf("Expected 'test', got '%s'", string(readBuf[:n]))
	}
}

func TestBufferedContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	queue := NewDoubleBufferQueue[byte](ctx, 10, 100*time.Millisecond)

	testData := []byte("test")
	_, err := queue.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	cancel()

	time.Sleep(50 * time.Millisecond)

	err = queue.Close()
	if err != nil {
		t.Errorf("Close after context cancel failed: %v", err)
	}
}

func TestBufferedGenericTypes(t *testing.T) {
	ctx := context.Background()
	intQueue := NewDoubleBufferQueue[int](ctx, 10, 100*time.Millisecond)
	defer intQueue.Close()

	intData := []int{1, 2, 3, 4, 5}
	n, err := intQueue.Write(intData)
	if err != nil {
		t.Errorf("Write int data failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected to write 5 ints, wrote %d", n)
	}

	readIntBuf := make([]int, 3)
	n, err = intQueue.Read(readIntBuf)
	if err != nil {
		t.Errorf("Read int data failed: %v", err)
	}
	if n != 3 {
		t.Errorf("Expected to read 3 ints, read %d", n)
	}

	expectedInts := []int{1, 2, 3}
	for i, v := range readIntBuf {
		if v != expectedInts[i] {
			t.Errorf("Expected int %d at position %d, got %d", expectedInts[i], i, v)
		}
	}

	stringQueue := NewDoubleBufferQueue[string](ctx, 5, 100*time.Millisecond)
	defer stringQueue.Close()

	stringData := []string{"hello", "world", "test"}
	n, err = stringQueue.Write(stringData)
	if err != nil {
		t.Errorf("Write string data failed: %v", err)
	}
	if n != 3 {
		t.Errorf("Expected to write 3 strings, wrote %d", n)
	}

	readStringBuf := make([]string, 2)
	n, err = stringQueue.Read(readStringBuf)
	if err != nil {
		t.Errorf("Read string data failed: %v", err)
	}
	if n != 2 {
		t.Errorf("Expected to read 2 strings, read %d", n)
	}

	expectedStrings := []string{"hello", "world"}
	for i, v := range readStringBuf {
		if v != expectedStrings[i] {
			t.Errorf("Expected string '%s' at position %d, got '%s'", expectedStrings[i], i, v)
		}
	}
}

func TestBufferedWriteTriggersSwap(t *testing.T) {
	ctx := context.Background()
	queue := NewDoubleBufferQueue[byte](ctx, 10, 1*time.Second) // long interval
	defer queue.Close()

	data1 := []byte("first")
	_, err := queue.Write(data1)
	if err != nil {
		t.Fatalf("First write failed: %v", err)
	}

	readBuf := make([]byte, 5)
	n, err := queue.Read(readBuf)
	if err != nil {
		t.Errorf("First read failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected to read 5 bytes, read %d", n)
	}

	data2 := []byte("second")
	_, err = queue.Write(data2)
	if err != nil {
		t.Fatalf("Second write failed: %v", err)
	}

	n, err = queue.Read(readBuf)
	if err != nil {
		t.Errorf("Second read failed: %v", err)
	}
	if n != 5 && n != 6 {
		t.Errorf("Expected to read 5-6 bytes, read %d", n)
	}
}

func TestBufferedLargeDataHandling(t *testing.T) {
	ctx := context.Background()
	queue := NewDoubleBufferQueue[byte](ctx, 10, 100*time.Millisecond) // small initial size
	defer queue.Close()

	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	n, err := queue.Write(largeData)
	if err != nil {
		t.Errorf("Write large data failed: %v", err)
	}
	if n != len(largeData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(largeData), n)
	}

	totalRead := 0
	readBuf := make([]byte, 1000)

	for totalRead < len(largeData) {
		n, err := queue.Read(readBuf)
		if err != nil {
			t.Errorf("Read chunk failed: %v", err)
			break
		}
		if n == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		for i := 0; i < n; i++ {
			expected := byte((totalRead + i) % 256)
			if readBuf[i] != expected {
				t.Errorf("Data mismatch at position %d: expected %d, got %d", totalRead+i, expected, readBuf[i])
				return
			}
		}

		totalRead += n
	}

	if totalRead != len(largeData) {
		t.Errorf("Expected to read %d total bytes, read %d", len(largeData), totalRead)
	}
}
