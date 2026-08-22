package frontend

import "time"

func (s *Shell) applyAudioSettings() {
	_ = s.settings.save()
	settings := s.currentAudioSettings()
	if backend, ok := s.backend.(AudioBackend); ok {
		if err := backend.ConfigureAudio(settings); err != nil {
			s.setStatus(s.tr("Audio settings: ") + err.Error())
			return
		}
	}
	s.audioMu.Lock()
	if s.audioOutput != nil {
		s.audioOutput.configure(settings)
	}
	s.audioMu.Unlock()
	s.setStatus(s.trf(
		"Audio: muted=%s volume=%d latency=%dms device=%s",
		s.tr(onOff(s.settings.Muted)),
		s.settings.Volume,
		s.settings.AudioLatencyMS,
		s.tr(s.audioDeviceLabel()),
	))
}

func (s *Shell) currentAudioSettings() AudioSettings {
	return AudioSettings{
		Muted:    s.settings.Muted,
		Volume:   s.settings.Volume,
		Latency:  time.Duration(s.settings.AudioLatencyMS) * time.Millisecond,
		DeviceID: s.settings.AudioDeviceID,
		MixMode:  s.settings.AudioMixMode,
		Soften:   s.settings.AudioSoften,
	}
}

// toggleAudioSoften turns the output-softening low-pass on or off. It is a pure
// playback filter (does not change the emulated audio), so it applies live.
func (s *Shell) toggleAudioSoften() {
	s.settings.AudioSoften = !s.settings.AudioSoften
	s.applyAudioSettings()
}

// toggleAudioMixMode switches between the faithful device policy (effects can
// silence the music, as on the handset) and the mixing policy (effects layer
// over a continuous background track). The change is baked into the core at
// creation, so it applies the next time a title is opened.
func (s *Shell) toggleAudioMixMode() {
	s.settings.AudioMixMode = !s.settings.AudioMixMode
	s.applyAudioSettings()
}

// audioMixModeLabel names the active audio policy for the settings row.
func (s *Shell) audioMixModeLabel() string {
	if s.settings.AudioMixMode {
		return s.tr("Mixed")
	}
	return s.tr("Faithful")
}

func (s *Shell) toggleMuted() {
	s.settings.Muted = !s.settings.Muted
	s.applyAudioSettings()
}

func (s *Shell) cycleVolume() {
	volume := s.settings.Volume + 5
	if volume > 100 {
		volume = 0
	}
	s.setVolume(volume)
}

func (s *Shell) setVolume(volume int) {
	if volume < 0 {
		volume = 0
	} else if volume > 100 {
		volume = 100
	}
	s.settings.Volume = volume
	s.applyAudioSettings()
}

func (s *Shell) cycleAudioLatency() {
	latency := s.settings.AudioLatencyMS + 10
	if latency > 250 {
		latency = 20
	}
	s.setAudioLatency(latency)
}

func (s *Shell) setAudioLatency(latencyMS int) {
	if latencyMS < 20 {
		latencyMS = 20
	} else if latencyMS > 250 {
		latencyMS = 250
	}
	s.settings.AudioLatencyMS = latencyMS
	s.applyAudioSettings()
}

func (s *Shell) cycleAudioDevice() {
	devices := s.audioDevices()
	current := -1
	for index, device := range devices {
		if device.ID == s.settings.AudioDeviceID {
			current = index
			break
		}
	}
	s.settings.AudioDeviceID = devices[(current+1)%len(devices)].ID
	s.applyAudioSettings()
}

func (s *Shell) audioDevices() []AudioDevice {
	devices := []AudioDevice{{Name: "System default"}}
	backend, ok := s.backend.(AudioDeviceBackend)
	if !ok {
		return devices
	}
	seen := map[string]bool{"": true}
	for _, device := range backend.AudioDevices() {
		if device.ID == "" || seen[device.ID] {
			continue
		}
		if device.Name == "" {
			device.Name = device.ID
		}
		seen[device.ID] = true
		devices = append(devices, device)
	}
	return devices
}

func (s *Shell) audioDeviceLabel() string {
	for _, device := range s.audioDevices() {
		if device.ID == s.settings.AudioDeviceID {
			return device.Name
		}
	}
	return "System default"
}
