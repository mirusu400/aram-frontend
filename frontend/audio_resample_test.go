package frontend

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestHostPCMDisplaySyncResamplingPreservesTone(t *testing.T) {
	// Display sync can stretch a 44.1 kHz stream to 0.96x. Holding each
	// source sample creates staircase noise even when no queue underruns.
	const rate = 42_336
	const frequency = 4_000.0
	const amplitude = 20_000.0
	pcm := make([]int16, rate/5)
	for i := range pcm {
		pcm[i] = int16(math.Round(amplitude * math.Sin(2*math.Pi*frequency*float64(i)/rate)))
	}
	var encoder hostPCMEncoder
	encoded, err := encoder.encode(AudioChunk{SampleRate: rate, Channels: 1, PCM16: pcm})
	if err != nil {
		t.Fatal(err)
	}
	var squaredError float64
	frames := len(encoded) / 4
	for frame := 1; frame < frames; frame++ {
		// A causal converter emits complete host periods, interpolating
		// between the previous source frame and the one just received.
		sourcePosition := float64(frame+1)*rate/hostAudioSampleRate - 1
		want := amplitude * math.Sin(2*math.Pi*frequency*sourcePosition/rate)
		got := float64(int16(binary.LittleEndian.Uint16(encoded[frame*4:])))
		squaredError += (got - want) * (got - want)
	}
	relativeRMS := math.Sqrt(squaredError/float64(frames-1)) / amplitude
	t.Logf("display-sync tone error: %.4f of amplitude", relativeRMS)
	if relativeRMS > 0.03 {
		t.Fatalf("display-sync tone error %.4f of amplitude, want <= 0.03", relativeRMS)
	}
}

func TestHostPCMInterpolatesAcrossSingleFrameChunks(t *testing.T) {
	var encoder hostPCMEncoder
	var got []int16
	for _, frame := range [][2]int16{{100, -100}, {-200, 200}, {300, -300}} {
		encoded, err := encoder.encode(AudioChunk{SampleRate: 22_050, Channels: 2, PCM16: frame[:]})
		if err != nil {
			t.Fatal(err)
		}
		for at := 0; at < len(encoded); at += 2 {
			got = append(got, int16(binary.LittleEndian.Uint16(encoded[at:])))
		}
	}
	want := []int16{100, -100, 100, -100, -50, 50, -200, 200, 50, -50, 300, -300}
	for i, sample := range want {
		if len(got) != len(want) || got[i] != sample {
			t.Fatalf("streamed stereo samples %v, want %v", got, want)
		}
	}
}

func TestHostPCMNativeRateIsExact(t *testing.T) {
	pcm := []int16{-32768, 32767, -1, 0, 1, 12345}
	var encoder hostPCMEncoder
	encoded, err := encoder.encode(AudioChunk{SampleRate: hostAudioSampleRate, Channels: 1, PCM16: pcm})
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != len(pcm)*4 {
		t.Fatalf("native-rate output has %d bytes", len(encoded))
	}
	for i, want := range pcm {
		for channel := 0; channel < 2; channel++ {
			if got := int16(binary.LittleEndian.Uint16(encoded[i*4+channel*2:])); got != want {
				t.Fatalf("native frame %d channel %d = %d, want %d", i, channel, got, want)
			}
		}
	}
}
