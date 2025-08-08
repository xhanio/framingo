package buffer

import (
	"io"
	"testing"
)

func TestNewPooledBuffer(t *testing.T) {
	buffer := NewPooledBuffer[byte](1024)
	if buffer == nil {
		t.Error("NewPooledBuffer() should return a non-nil buffer")
	}
	
	if buffer.Closed() {
		t.Error("New buffer should not be closed")
	}
	
	if buffer.Len() != 0 {
		t.Errorf("New buffer length should be 0, got %d", buffer.Len())
	}
	
	if buffer.Available() != 0 {
		t.Errorf("New buffer available should be 0, got %d", buffer.Available())
	}
}

func TestPooledBufferWrite(t *testing.T) {
	buffer := NewPooledBuffer[byte](10)
	
	// Test normal write
	data := []byte("hello")
	n, err := buffer.Write(data)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}
	if buffer.Len() != len(data) {
		t.Errorf("Buffer length should be %d, got %d", len(data), buffer.Len())
	}
	
	// Test write empty data
	n, err = buffer.Write([]byte{})
	if err != nil {
		t.Errorf("Write empty failed: %v", err)
	}
	if n != 0 {
		t.Errorf("Expected to write 0 bytes, wrote %d", n)
	}
	
	// Test expansion
	moreData := []byte(" world and more data to trigger expansion")
	n, err = buffer.Write(moreData)
	if err != nil {
		t.Errorf("Write with expansion failed: %v", err)
	}
	if n != len(moreData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(moreData), n)
	}
	
	expectedLen := len(data) + len(moreData)
	if buffer.Len() != expectedLen {
		t.Errorf("Buffer length should be %d, got %d", expectedLen, buffer.Len())
	}
}

func TestPooledBufferRead(t *testing.T) {
	buffer := NewPooledBuffer[byte](10)
	testData := []byte("hello world")
	
	// Write test data
	_, err := buffer.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	
	// Test normal read
	readBuf := make([]byte, 5)
	n, err := buffer.Read(readBuf)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected to read 5 bytes, read %d", n)
	}
	if string(readBuf) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", string(readBuf))
	}
	if buffer.Available() != 6 { // " world" remains
		t.Errorf("Available should be 6, got %d", buffer.Available())
	}
	
	// Test read remaining data
	readBuf2 := make([]byte, 10)
	n, err = buffer.Read(readBuf2)
	if err != nil {
		t.Errorf("Read remaining failed: %v", err)
	}
	if n != 6 {
		t.Errorf("Expected to read 6 bytes, read %d", n)
	}
	if string(readBuf2[:n]) != " world" {
		t.Errorf("Expected ' world', got '%s'", string(readBuf2[:n]))
	}
	
	// Test read at EOF
	n, err = buffer.Read(readBuf2)
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
	if n != 0 {
		t.Errorf("Expected to read 0 bytes at EOF, read %d", n)
	}
	
	// Test read empty buffer
	readBuf3 := make([]byte, 0)
	n, err = buffer.Read(readBuf3)
	if err != nil {
		t.Errorf("Read empty buffer failed: %v", err)
	}
	if n != 0 {
		t.Errorf("Expected to read 0 bytes from empty buffer, read %d", n)
	}
}

func TestPooledBufferSeek(t *testing.T) {
	buffer := NewPooledBuffer[byte](10)
	testData := []byte("hello world")
	buffer.Write(testData)
	
	tests := []struct {
		name     string
		offset   int64
		whence   int
		expected int64
		wantErr  bool
	}{
		{"seek start", 5, io.SeekStart, 5, false},
		{"seek current forward", 2, io.SeekCurrent, 7, false},
		{"seek current backward", -3, io.SeekCurrent, 4, false},
		{"seek end", 0, io.SeekEnd, 11, false},
		{"seek end with offset", -5, io.SeekEnd, 6, false},
		{"seek negative", -1, io.SeekStart, 0, true},
		{"seek invalid whence", 0, 999, 0, true},
		{"seek beyond end", 20, io.SeekStart, 11, false}, // clamped to length
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, err := buffer.Seek(tt.offset, tt.whence)
			
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
			
			if pos != tt.expected {
				t.Errorf("Expected position %d, got %d", tt.expected, pos)
			}
		})
	}
}

func TestPooledBufferReset(t *testing.T) {
	buffer := NewPooledBuffer[byte](10)
	testData := []byte("hello world")
	
	// Write and read some data
	buffer.Write(testData)
	readBuf := make([]byte, 5)
	buffer.Read(readBuf)
	
	if buffer.Len() != 11 {
		t.Errorf("Expected length 11, got %d", buffer.Len())
	}
	if buffer.Available() != 6 {
		t.Errorf("Expected available 6, got %d", buffer.Available())
	}
	
	// Reset should clear data and position
	buffer.Reset()
	
	if buffer.Len() != 0 {
		t.Errorf("After reset, length should be 0, got %d", buffer.Len())
	}
	if buffer.Available() != 0 {
		t.Errorf("After reset, available should be 0, got %d", buffer.Available())
	}
	
	// Should be able to write again
	newData := []byte("new data")
	n, err := buffer.Write(newData)
	if err != nil {
		t.Errorf("Write after reset failed: %v", err)
	}
	if n != len(newData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(newData), n)
	}
}

func TestPooledBufferResetRead(t *testing.T) {
	buffer := NewPooledBuffer[byte](10)
	testData := []byte("hello")
	
	// Write and read some data
	buffer.Write(testData)
	readBuf := make([]byte, 3)
	buffer.Read(readBuf) // read "hel"
	
	if buffer.Available() != 2 {
		t.Errorf("Expected available 2, got %d", buffer.Available())
	}
	
	// Reset read position only
	buffer.ResetRead()
	
	if buffer.Len() != 5 {
		t.Errorf("After ResetRead, length should still be 5, got %d", buffer.Len())
	}
	if buffer.Available() != 5 {
		t.Errorf("After ResetRead, available should be 5, got %d", buffer.Available())
	}
	
	// Should be able to read from beginning again
	fullReadBuf := make([]byte, 5)
	n, err := buffer.Read(fullReadBuf)
	if err != nil {
		t.Errorf("Read after ResetRead failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected to read 5 bytes, read %d", n)
	}
	if string(fullReadBuf) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", string(fullReadBuf))
	}
}

func TestPooledBufferClose(t *testing.T) {
	buffer := NewPooledBuffer[byte](10)
	testData := []byte("hello")
	buffer.Write(testData)
	
	// Close the buffer
	err := buffer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	
	if !buffer.Closed() {
		t.Error("Buffer should be closed")
	}
	
	// Operations should fail after close
	_, err = buffer.Write([]byte("test"))
	if err == nil {
		t.Error("Write should fail on closed buffer")
	}
	
	readBuf := make([]byte, 5)
	_, err = buffer.Read(readBuf)
	if err == nil {
		t.Error("Read should fail on closed buffer")
	}
	
	_, err = buffer.Seek(0, io.SeekStart)
	if err == nil {
		t.Error("Seek should fail on closed buffer")
	}
	
	// Double close should not error
	err = buffer.Close()
	if err != nil {
		t.Errorf("Double close should not error, got: %v", err)
	}
	
	// Properties should return zero/empty after close
	if buffer.Len() != 0 {
		t.Errorf("Closed buffer Len() should be 0, got %d", buffer.Len())
	}
	if buffer.Available() != 0 {
		t.Errorf("Closed buffer Available() should be 0, got %d", buffer.Available())
	}
	if buffer.Data() != nil {
		t.Error("Closed buffer Data() should return nil")
	}
}

func TestPooledBufferData(t *testing.T) {
	buffer := NewPooledBuffer[byte](10)
	
	// Empty buffer
	data := buffer.Data()
	if data != nil {
		t.Error("Empty buffer Data() should return nil")
	}
	
	// With data
	testData := []byte("hello world")
	buffer.Write(testData)
	
	data = buffer.Data()
	if len(data) != len(testData) {
		t.Errorf("Data length should be %d, got %d", len(testData), len(data))
	}
	if string(data) != string(testData) {
		t.Errorf("Expected '%s', got '%s'", string(testData), string(data))
	}
	
	// Modifying returned data should not affect buffer (it's a copy)
	data[0] = 'H'
	originalData := buffer.Data()
	if originalData[0] != 'h' {
		t.Error("Buffer data should not be modified when returned data is changed")
	}
}

func TestPooledBufferExpansion(t *testing.T) {
	buffer := NewPooledBuffer[byte](5) // small initial size
	
	originalCap := buffer.Cap()
	
	// Write data that exceeds capacity
	largeData := make([]byte, originalCap*3) // Make sure it's larger than original capacity
	for i := range largeData {
		largeData[i] = byte('a' + (i % 26))
	}
	
	n, err := buffer.Write(largeData)
	if err != nil {
		t.Errorf("Write large data failed: %v", err)
	}
	if n != len(largeData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(largeData), n)
	}
	
	// Buffer should have expanded
	if buffer.Cap() <= originalCap {
		t.Errorf("Buffer should have expanded, original cap %d, new cap %d", originalCap, buffer.Cap())
	}
	
	// Data should be preserved
	if buffer.Len() != len(largeData) {
		t.Errorf("Buffer length should be %d, got %d", len(largeData), buffer.Len())
	}
	
	readData := buffer.Data()
	if len(readData) != len(largeData) {
		t.Errorf("Read data length should be %d, got %d", len(largeData), len(readData))
	}
	
	for i, b := range readData {
		if b != largeData[i] {
			t.Errorf("Data mismatch at position %d: expected %c, got %c", i, largeData[i], b)
			break
		}
	}
}

func TestPooledBufferGenericTypes(t *testing.T) {
	// Test with int buffer
	intBuffer := NewPooledBuffer[int](10)
	
	intData := []int{1, 2, 3, 4, 5}
	n, err := intBuffer.Write(intData)
	if err != nil {
		t.Errorf("Write int data failed: %v", err)
	}
	if n != len(intData) {
		t.Errorf("Expected to write %d ints, wrote %d", len(intData), n)
	}
	
	readIntBuf := make([]int, 3)
	n, err = intBuffer.Read(readIntBuf)
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
	
	// Test with string buffer
	stringBuffer := NewPooledBuffer[string](5)
	
	stringData := []string{"hello", "world", "test"}
	n, err = stringBuffer.Write(stringData)
	if err != nil {
		t.Errorf("Write string data failed: %v", err)
	}
	if n != len(stringData) {
		t.Errorf("Expected to write %d strings, wrote %d", len(stringData), n)
	}
	
	readStringBuf := make([]string, 2)
	n, err = stringBuffer.Read(readStringBuf)
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