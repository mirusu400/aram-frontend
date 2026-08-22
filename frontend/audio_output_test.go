package frontend

import (
	"encoding/binary"
	"io"
	"testing"
	"time"
)

func TestEncodeHostPCMConvertsMonoAndSampleRate(t *testing.T) {
	encoded, err := encodeHostPCM(AudioChunk{
		SampleRate: 22_050,
		Channels:   1,
		PCM16:      []int16{100, -200},
	})
	if err != nil {
		t.Fatalf("encodeHostPCM: %v", err)
	}
	if len(encoded) != 4*4 {
		t.Fatalf("encoded bytes = %d, want 16", len(encoded))
	}
	want := []int16{100, 100, 100, 100, -200, -200, -200, -200}
	for index, expected := range want {
		got := int16(binary.LittleEndian.Uint16(encoded[index*2:]))
		if got != expected {
			t.Fatalf("sample %d = %d, want %d", index, got, expected)
		}
	}
}

func TestPCMQueuePadsUnderrunWithSilence(t *testing.T) {
	queue := newPCMQueue(32)
	queue.enqueue([]byte{1, 2, 3, 4})
	destination := make([]byte, 8)
	count, err := queue.Read(destination)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if count != len(destination) {
		t.Fatalf("Read count = %d, want %d", count, len(destination))
	}
	want := []byte{1, 2, 3, 4, 0, 0, 0, 0}
	for index := range want {
		if destination[index] != want[index] {
			t.Fatalf("byte %d = %d, want %d", index, destination[index], want[index])
		}
	}
}

func TestPCMQueueDropsOldestStereoFrames(t *testing.T) {
	queue := newPCMQueue(8)
	queue.enqueue([]byte{1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3})
	if got := queue.availableBytes(); got != 8 {
		t.Fatalf("available bytes = %d, want 8", got)
	}
	destination := make([]byte, 8)
	if _, err := queue.Read(destination); err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := []byte{2, 2, 2, 2, 3, 3, 3, 3}
	for index := range want {
		if destination[index] != want[index] {
			t.Fatalf("byte %d = %d, want %d", index, destination[index], want[index])
		}
	}
	if got := queue.availableBytes(); got != 0 {
		t.Fatalf("available bytes after read = %d, want 0", got)
	}
	queue.close()
	if _, err := queue.Read(make([]byte, 4)); err != io.EOF {
		t.Fatalf("closed empty queue error = %v, want EOF", err)
	}
}

func TestAudioQueueBytesBoundsLatency(t *testing.T) {
	low := audioQueueBytes(20 * time.Millisecond)
	high := audioQueueBytes(250 * time.Millisecond)
	if low != hostAudioSampleRate*4*220/1000 {
		t.Fatalf("low-latency queue = %d", low)
	}
	if high != hostAudioSampleRate*4*700/1000 {
		t.Fatalf("high-latency queue = %d", high)
	}
}
