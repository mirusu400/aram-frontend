package frontend

import (
	"bytes"
	"runtime/pprof"
	"sync"
)

// cpuProfileState holds a continuously running CPU profile that the debug
// bundle can snapshot on demand. Profiling carries real runtime overhead, so it
// is opt-in through settings and off by default.
type cpuProfileState struct {
	mu      sync.Mutex
	buf     *bytes.Buffer
	running bool
}

// setCPUProfiling starts or stops CPU profiling into an in-memory buffer and
// reports the state actually reached (a failed start reports false).
func (s *Shell) setCPUProfiling(enabled bool) bool {
	s.cpuProfile.mu.Lock()
	defer s.cpuProfile.mu.Unlock()
	if enabled == s.cpuProfile.running {
		return s.cpuProfile.running
	}
	if enabled {
		buffer := &bytes.Buffer{}
		if err := pprof.StartCPUProfile(buffer); err != nil {
			return false
		}
		s.cpuProfile.buf = buffer
		s.cpuProfile.running = true
		return true
	}
	pprof.StopCPUProfile()
	s.cpuProfile.buf = nil
	s.cpuProfile.running = false
	return false
}

// snapshotCPUProfile finalizes the running profile, returns its bytes, and
// restarts a fresh one so the next export covers only new activity. It returns
// nil when profiling is off. A finalized profile is always readable by
// `go tool pprof`, so this is safe to call on the crash path as well.
func (s *Shell) snapshotCPUProfile() []byte {
	s.cpuProfile.mu.Lock()
	defer s.cpuProfile.mu.Unlock()
	if !s.cpuProfile.running || s.cpuProfile.buf == nil {
		return nil
	}
	pprof.StopCPUProfile()
	data := append([]byte(nil), s.cpuProfile.buf.Bytes()...)
	buffer := &bytes.Buffer{}
	if err := pprof.StartCPUProfile(buffer); err != nil {
		s.cpuProfile.buf = nil
		s.cpuProfile.running = false
		return data
	}
	s.cpuProfile.buf = buffer
	return data
}

// toggleCPUProfile flips the persisted CPU-profiling preference and applies it.
func (s *Shell) toggleCPUProfile() {
	enabled := s.setCPUProfiling(!s.settings.CPUProfile)
	s.settings.CPUProfile = enabled
	_ = s.settings.save()
	if enabled {
		s.setStatus(s.tr("CPU profiling on - captured in the next debug bundle"))
		return
	}
	s.setStatus(s.tr("CPU profiling off"))
}
