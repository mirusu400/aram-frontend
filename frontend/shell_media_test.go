package frontend

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"
)

type videoBackend struct {
	frame  VideoFrame
	chunks []AudioChunk
	drains int
}

func (*videoBackend) Open(context.Context, OpenRequest) (InputInfo, error) {
	return InputInfo{DisplayName: "synthetic.dat"}, nil
}

func (*videoBackend) State() BackendState          { return StateRunning }
func (*videoBackend) Supports(BackendCommand) bool { return true }

func (*videoBackend) Execute(context.Context, BackendCommand) error { return nil }

func (*videoBackend) Close() error { return nil }

func (backend *videoBackend) VideoFrame() VideoFrame { return backend.frame }

func (backend *videoBackend) DrainAudio() AudioChunk {
	backend.drains++
	if len(backend.chunks) == 0 {
		return AudioChunk{}
	}
	chunk := backend.chunks[0]
	backend.chunks = backend.chunks[1:]
	return chunk
}

func guestFrame(width, height int, shade uint8) *image.RGBA {
	frame := image.NewRGBA(image.Rect(0, 0, width, height))
	frame.SetRGBA(0, 0, color.RGBA{R: shade, G: shade, B: shade, A: 0xff})
	return frame
}

// A guest keeps its screen size for the whole run, so publishing a new frame
// must reuse the texture rather than allocate and discard one per frame.
func TestGuestFrameUploadReusesTheTextureUntilTheResolutionChanges(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	backend := &videoBackend{}
	shell := NewShell(backend, nil, "")

	backend.frame = VideoFrame{Image: guestFrame(24, 32, 0x10), Sequence: 1}
	shell.updateVideo()
	first := shell.frameImage
	if first == nil {
		t.Fatal("no guest texture was created for the first frame")
	}

	backend.frame = VideoFrame{Image: guestFrame(24, 32, 0x20), Sequence: 2}
	shell.updateVideo()
	if shell.frameImage != first {
		t.Fatal("a same-sized guest frame rebuilt the texture")
	}
	if shell.frame.Sequence != 2 {
		t.Fatalf("published frame sequence = %d, want 2", shell.frame.Sequence)
	}

	backend.frame = VideoFrame{Image: guestFrame(48, 64, 0x30), Sequence: 3}
	shell.updateVideo()
	if shell.frameImage == first {
		t.Fatal("a resized guest frame kept the old texture")
	}
	if bounds := shell.frameImage.Bounds(); bounds.Dx() != 48 || bounds.Dy() != 64 {
		t.Fatalf("resized guest texture bounds = %v", bounds)
	}
}

func TestGuestFrameUploadIgnoresRepeatedSequences(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	backend := &videoBackend{}
	shell := NewShell(backend, nil, "")

	backend.frame = VideoFrame{Image: guestFrame(24, 32, 0x10), Sequence: 4}
	shell.updateVideo()
	published := shell.frame.Image

	backend.frame = VideoFrame{Image: guestFrame(24, 32, 0x40), Sequence: 4}
	shell.updateVideo()
	if shell.frame.Image != published {
		t.Fatal("an unchanged sequence republished the guest frame")
	}
}

func TestVideoUpdatePublishesGuestTimelineAnchor(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	backend := &videoBackend{frame: VideoFrame{
		Image:      guestFrame(24, 32, 0x10),
		Sequence:   2,
		GuestNS:    int64(750 * time.Millisecond),
		Generation: 5,
	}}
	shell := NewShell(backend, nil, "")
	shell.updateVideo()
	if got := shell.latestVideoGuestNS.Load(); got != int64(750*time.Millisecond) {
		t.Fatalf("video guest anchor = %d", got)
	}
	if got := shell.latestVideoGeneration.Load(); got != 5 {
		t.Fatalf("video generation = %d", got)
	}
}

// The backend already owns tightly packed pixels, so the upload must hand that
// buffer straight to the GPU instead of copying it once per frame.
func TestGuestFramePixelsUploadsPackedFramesWithoutCopying(t *testing.T) {
	frame := guestFrame(8, 4, 0x55)
	var scratch *image.RGBA
	pixels := guestFramePixels(frame, &scratch)
	if len(pixels) != len(frame.Pix) || &pixels[0] != &frame.Pix[0] {
		t.Fatal("a packed guest frame was copied before upload")
	}
	if scratch != nil {
		t.Fatal("a packed guest frame allocated a scratch buffer")
	}
}

func TestGuestFramePixelsConvertsUnpackedFramesThroughOneScratchBuffer(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 6, 3))
	source.SetNRGBA(1, 1, color.NRGBA{R: 0xff, A: 0xff})
	var scratch *image.RGBA

	pixels := guestFramePixels(source, &scratch)
	if scratch == nil {
		t.Fatal("an unpacked guest frame did not use a scratch buffer")
	}
	if want := 6 * 3 * 4; len(pixels) != want {
		t.Fatalf("converted pixel length = %d, want %d", len(pixels), want)
	}
	if got := scratch.RGBAAt(1, 1); got.R != 0xff || got.A != 0xff {
		t.Fatalf("converted pixel = %+v", got)
	}

	reused := scratch
	source.SetNRGBA(2, 2, color.NRGBA{B: 0xff, A: 0xff})
	if guestFramePixels(source, &scratch); scratch != reused {
		t.Fatal("a second unpacked guest frame reallocated the scratch buffer")
	}
}

// A title heavy enough to keep a frame batch pending across ticks is exactly
// the one whose audio queue must keep being drained.
func TestAudioDrainsWhileAFrameBatchIsStillPending(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("APPDATA", temporary)
	t.Setenv("XDG_CONFIG_HOME", temporary)
	backend := &videoBackend{
		chunks: []AudioChunk{{
			SampleRate: hostAudioSampleRate,
			Channels:   2,
			PCM16:      make([]int16, 512),
		}},
	}
	shell := NewShell(backend, nil, "")
	shell.input = &InputInfo{DisplayName: "synthetic.dat"}
	shell.audioOutput = &audioOutput{queue: newPCMQueue(64 * 1024)}
	shell.frameRunPending = true

	shell.updateAudio()

	if backend.drains == 0 {
		t.Fatal("audio was not drained while a frame batch was pending")
	}
	if got := shell.audioOutput.queue.availableBytes(); got == 0 {
		t.Fatal("no guest audio reached the host queue")
	}
}
