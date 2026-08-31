package frontend

import (
	"image"
	"image/draw"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func (s *Shell) closeInput() {
	if err := s.releaseCurrentInput(); err != nil {
		s.setStatus(s.tr("Close: ") + err.Error())
		return
	}
	s.setStatus(s.tr("Title closed"))
}

func (s *Shell) releaseCurrentInput() error {
	if err := s.backend.Close(); err != nil {
		return err
	}
	s.audioMu.Lock()
	s.audioSuspended = true
	if s.audioOutput != nil {
		s.audioOutput.flush()
	}
	s.audioMu.Unlock()
	if s.temporaryPath != "" {
		removeTemporaryDrop(s.temporaryPath)
		s.temporaryPath = ""
	}
	s.input = nil
	s.selectedPath = ""
	s.problem = nil
	s.hostPaused = false
	s.frameGeneration++
	s.frameRunPending = false
	s.clearMeasuredSpeed()
	s.frame = VideoFrame{}
	s.frameImage = nil
	s.frameScratch = nil
	s.releaseDisplaySurfaces()
	s.state = FrontendEmpty
	setPlatformWindowTitle(s.tr("ARAM - Archived Runtime for ARM Mobiles"))
	return nil
}

// updateAudio moves guest PCM into the host queue once per tick.
//
// It deliberately runs while a frame batch is still in flight. Skipping the
// drain until the batch landed meant a title heavy enough to keep a batch
// pending across ticks - exactly the title whose audio is already fragile -
// stopped being drained at all, so the host queue underran and the sound
// broke up. Draining is safe mid-batch because the backend serializes its own
// audio buffer.
func (s *Shell) updateAudio() {
	if s.loading || s.input == nil {
		return
	}
	s.drainAudioOnce(true)
}

// drainAudioOnce moves produced guest PCM into the output queue once. It is the
// shared body of the main-thread Update drain (allowCreate=true, which may build
// the output because that needs the host audio context) and the audio pump
// goroutine (allowCreate=false, which only feeds an output that already exists).
// audioMu serialises the drain+enqueue so the two callers can never interleave a
// chunk out of order, and so a stalled or jittery 60Hz video tick can no longer
// starve the queue: the pump keeps feeding on its own fine cadence.
func (s *Shell) drainAudioOnce(allowCreate bool) {
	backend, ok := s.backend.(AudioStreamBackend)
	if !ok {
		return
	}
	s.audioMu.Lock()
	defer s.audioMu.Unlock()
	if s.audioSuspended {
		return
	}
	if s.audioOutput == nil && !allowCreate {
		return
	}
	for taken := 0; taken < maxAudioChunksPerDrain; taken++ {
		chunk := backend.DrainAudio()
		if len(chunk.PCM16) == 0 {
			break
		}
		if s.audioOutput == nil {
			output, err := newAudioOutput(s.currentAudioSettings())
			if err != nil {
				s.appendLog(s.tr("Audio output: ") + err.Error())
				return
			}
			s.audioOutput = output
			s.startAudioPumpLocked()
		}
		if err := s.audioOutput.enqueue(
			chunk,
			s.latestVideoGuestNS.Load(),
			s.latestVideoGeneration.Load(),
		); err != nil && allowCreate {
			// Only the main-thread caller touches s.logs; the pump goroutine
			// must not. A rare encode error still surfaces on the next Update
			// tick.
			s.appendLog(s.tr("Audio stream: ") + err.Error())
		}
	}
	if s.audioOutput != nil {
		now := time.Now()
		s.audioOutput.startIfReady(now)
		s.audioOutput.maybeSample(now)
	}
}

// maxAudioChunksPerDrain bounds one drain pass.
//
// A backend publishes a chunk per service advance, and a title that parks on
// timers splits one presentation quantum into several of them. Taking a single
// chunk per pass therefore consumed guest audio more slowly than the guest
// produced it, and the backend's own retention window silently dropped the
// excess - a starvation that grows worse exactly when a title is already
// struggling. The bound still keeps one pass short so the lock is never held
// for long.
const maxAudioChunksPerDrain = 16

// startAudioPumpLocked launches the audio pump goroutine the first time an
// output exists. The pump feeds produced PCM into the queue every few
// milliseconds, decoupled from the 60Hz Update loop, so a stalled or jittery
// video tick cannot starve the sound. It idles harmlessly (DrainAudio returns
// nothing) whenever no title is running. Caller holds audioMu.
func (s *Shell) startAudioPumpLocked() {
	if s.audioPumpStarted {
		return
	}
	s.audioPumpStarted = true
	go func() {
		ticker := time.NewTicker(4 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			s.drainAudioOnce(false)
		}
	}()
}

func (s *Shell) closeAudio() error {
	s.audioMu.Lock()
	defer s.audioMu.Unlock()
	s.audioSuspended = true
	if s.audioOutput == nil {
		return nil
	}
	err := s.audioOutput.close()
	s.audioOutput = nil
	return err
}

func isAudioDiscontinuityCommand(command BackendCommand) bool {
	switch command {
	case CommandStart,
		CommandPauseResume,
		CommandStop,
		CommandReset,
		CommandFastForward,
		CommandLoadState,
		CommandRewind:
		return true
	default:
		return false
	}
}

// beginAudioDiscontinuity blocks both audio drainers and clears host-owned
// PCM before a lifecycle or timeline command can move guest time elsewhere.
func (s *Shell) beginAudioDiscontinuity() {
	s.audioMu.Lock()
	defer s.audioMu.Unlock()
	s.audioSuspended = true
	if s.audioOutput != nil {
		s.audioOutput.flush()
	}
}

// finishAudioDiscontinuity clears any PCM that raced with the command and only
// resumes draining when the resulting machine state is actually running.
func (s *Shell) finishAudioDiscontinuity(state BackendState) {
	s.audioMu.Lock()
	defer s.audioMu.Unlock()
	if s.audioOutput != nil {
		s.audioOutput.flush()
	}
	s.audioSuspended = state != StateRunning
}

func (s *Shell) flushAudioDiscontinuity() {
	s.audioMu.Lock()
	defer s.audioMu.Unlock()
	if s.audioOutput != nil {
		s.audioOutput.flush()
	}
}

func (s *Shell) audioQueueTelemetry() AudioQueueTelemetry {
	s.audioMu.Lock()
	defer s.audioMu.Unlock()
	if s.audioOutput == nil {
		return AudioQueueTelemetry{}
	}
	return s.audioOutput.telemetry()
}

// audioTraceRender returns the rendered audio event trace for the debug bundle,
// or nil when no output has been created (no audio has played this session).
func (s *Shell) audioTraceRender() []byte {
	s.audioMu.Lock()
	defer s.audioMu.Unlock()
	if s.audioOutput == nil {
		return nil
	}
	return s.audioOutput.trace.render()
}

func (s *Shell) syncBackendState() {
	if s.loading || s.dialogOpen || s.problem != nil || s.input == nil {
		return
	}
	state := frontendStateForBackend(s.backend.State())
	if state == FrontendEmpty {
		state = FrontendReady
	}
	// Announce when the guest ends on its own (for example a Clet calling
	// MC_knlExit). Without this the screen freezes on the last frame with no
	// hint that the title exited or how to relaunch it.
	if state == FrontendStopped && s.lastRunState != FrontendStopped {
		s.setStatus(s.tr("Title exited - press Start to restart"))
	}
	s.lastRunState = state
	s.state = state
}

func (s *Shell) syncHostLifecycle() {
	s.hostActive = s.hostActiveRequest.Load()
	state := s.backend.State()
	if !s.hostActive &&
		!s.hostPaused &&
		state == StateRunning &&
		!s.busyCommands[CommandPauseResume] {
		s.hostPaused = true
		s.executeBackend(CommandPauseResume)
		return
	}
	if s.hostActive &&
		s.hostPaused &&
		state == StatePaused &&
		!s.busyCommands[CommandPauseResume] {
		s.hostPaused = false
		s.executeBackend(CommandPauseResume)
	}
}

func (s *Shell) stableState() FrontendState {
	if s.input == nil {
		return FrontendEmpty
	}
	state := frontendStateForBackend(s.backend.State())
	if state == FrontendEmpty {
		return FrontendReady
	}
	return state
}

func (s *Shell) currentFrame() VideoFrame {
	return s.frame
}

func (s *Shell) updateVideo() {
	backend, ok := s.backend.(VideoBackend)
	if !ok {
		return
	}
	frame := backend.VideoFrame()
	if frame.Image == nil {
		return
	}
	bounds := frame.Image.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	s.latestVideoGuestNS.Store(frame.GuestNS)
	s.latestVideoGeneration.Store(frame.Generation)
	if s.frameImage != nil && frame.Sequence == s.frame.Sequence {
		return
	}
	s.frame = frame
	s.uploadGuestFrame(frame.Image)
}

// uploadGuestFrame publishes guest pixels into a persistent GPU image.
//
// Building a fresh ebiten.Image per frame allocated and then discarded a
// texture sixty times a second, which is pure churn: the guest screen keeps
// its size for the whole run. The texture is now reused and only rebuilt when
// the guest actually changes resolution.
func (s *Shell) uploadGuestFrame(source image.Image) {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if s.frameImage == nil ||
		s.frameImage.Bounds().Dx() != width ||
		s.frameImage.Bounds().Dy() != height {
		s.frameImage = ebiten.NewImage(width, height)
	}
	s.frameImage.WritePixels(guestFramePixels(source, &s.frameScratch))
}

// guestFramePixels returns the frame as tightly packed premultiplied RGBA.
//
// Backends hand over an *image.RGBA whose rows are already contiguous, so the
// common path uploads the backend's own buffer with no copy at all. Any other
// image type is converted through a scratch buffer that is reused across
// frames rather than reallocated.
func guestFramePixels(source image.Image, scratch **image.RGBA) []byte {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	stride := width * 4
	if rgba, ok := source.(*image.RGBA); ok &&
		rgba.Stride == stride &&
		len(rgba.Pix) == stride*height {
		return rgba.Pix
	}
	if *scratch == nil ||
		(*scratch).Bounds().Dx() != width ||
		(*scratch).Bounds().Dy() != height {
		*scratch = image.NewRGBA(image.Rect(0, 0, width, height))
	}
	draw.Draw(*scratch, (*scratch).Bounds(), source, bounds.Min, draw.Src)
	return (*scratch).Pix
}
