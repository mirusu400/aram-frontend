package frontend

import (
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
	if s.audioOutput != nil {
		s.audioOutput.flush()
	}
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
	s.state = FrontendEmpty
	setPlatformWindowTitle(s.tr("ARAM - Archived Runtime for ARM Mobiles"))
	return nil
}

func (s *Shell) updateAudio() {
	if s.loading || s.input == nil || s.frameRunPending {
		return
	}
	backend, ok := s.backend.(AudioStreamBackend)
	if !ok {
		return
	}
	chunk := backend.DrainAudio()
	if len(chunk.PCM16) == 0 {
		if s.audioOutput != nil {
			s.audioOutput.startIfReady(time.Now())
		}
		return
	}
	if s.audioOutput == nil {
		output, err := newAudioOutput(s.currentAudioSettings())
		if err != nil {
			s.appendLog(s.tr("Audio output: ") + err.Error())
			return
		}
		s.audioOutput = output
	}
	if err := s.audioOutput.enqueue(chunk); err != nil {
		s.appendLog(s.tr("Audio stream: ") + err.Error())
	}
	s.audioOutput.startIfReady(time.Now())
}

func (s *Shell) closeAudio() error {
	if s.audioOutput == nil {
		return nil
	}
	err := s.audioOutput.close()
	s.audioOutput = nil
	return err
}

func (s *Shell) syncBackendState() {
	if s.loading || s.dialogOpen || s.problem != nil || s.input == nil {
		return
	}
	state := frontendStateForBackend(s.backend.State())
	if state == FrontendEmpty {
		state = FrontendReady
	}
	s.state = state
}

func (s *Shell) syncHostLifecycle() {
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
	if s.frameImage != nil && frame.Sequence == s.frame.Sequence {
		return
	}
	s.frame = frame
	s.frameImage = ebiten.NewImageFromImage(frame.Image)
}
