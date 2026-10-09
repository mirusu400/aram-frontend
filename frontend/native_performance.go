package frontend

import "sync"

// NativePerformanceHost runs on the locked emulation/audio OS thread. A host
// may attach priorities and performance hints without changing guest time.
// Each completion uses the same host as the matching start, even on reattach.
type NativePerformanceHost interface {
	FrameWorkStarted(targetNanoseconds int64, uiPriority bool)
	FrameWorkFinished(actualNanoseconds int64)
	PrepareAudioThread()
}

var nativePerformanceBridge struct {
	sync.RWMutex
	host NativePerformanceHost
}

func SetNativePerformanceHost(host NativePerformanceHost) {
	nativePerformanceBridge.Lock()
	nativePerformanceBridge.host = host
	nativePerformanceBridge.Unlock()
}

func currentNativePerformanceHost() NativePerformanceHost {
	nativePerformanceBridge.RLock()
	defer nativePerformanceBridge.RUnlock()
	return nativePerformanceBridge.host
}
