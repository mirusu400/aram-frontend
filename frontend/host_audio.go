package frontend

import (
	"fmt"
	"sync"
	"time"
)

// HostAudioProperties are Android's reported output defaults, not measured
// device latency or Oboe's negotiated format. Zero keeps portable buffering.
type HostAudioProperties struct {
	SampleRate      int
	FramesPerBuffer int
}

var hostAudioProperties struct {
	sync.RWMutex
	value HostAudioProperties
}

func SetHostAudioProperties(sampleRate, framesPerBuffer int) {
	value := HostAudioProperties{SampleRate: sampleRate, FramesPerBuffer: framesPerBuffer}
	if value.burst() == 0 {
		value = HostAudioProperties{}
	}
	hostAudioProperties.Lock()
	hostAudioProperties.value = value
	hostAudioProperties.Unlock()
}

func currentHostAudioProperties() HostAudioProperties {
	hostAudioProperties.RLock()
	defer hostAudioProperties.RUnlock()
	return hostAudioProperties.value
}

func (p HostAudioProperties) burst() time.Duration {
	if p.SampleRate < hostAudioMinSampleRate || p.SampleRate > hostAudioMaxSampleRate ||
		p.FramesPerBuffer <= 0 || p.FramesPerBuffer > p.SampleRate/25 {
		return 0
	}
	return time.Duration(p.FramesPerBuffer) * time.Second / time.Duration(p.SampleRate)
}

type audioBufferPlan struct {
	queueTarget time.Duration
	player      time.Duration
	pump        time.Duration
}

func planAudioBuffers(latency time.Duration, properties HostAudioProperties) audioBufferPlan {
	latency = normalizedAudioLatency(latency)
	burst := properties.burst()
	if burst == 0 {
		return audioBufferPlan{queueTarget: latency, player: latency, pump: 4 * time.Millisecond}
	}
	// Budget the Android queue and player read-ahead together instead of
	// giving each the entire requested latency. Round each half up to a burst.
	half := (latency + 1) / 2
	buffer := ((half + burst - 1) / burst) * burst
	return audioBufferPlan{
		queueTarget: buffer,
		player:      buffer,
		pump:        max(2*time.Millisecond, min(8*time.Millisecond, burst/2)),
	}
}

func hostAudioPumpInterval() time.Duration {
	return planAudioBuffers(60*time.Millisecond, currentHostAudioProperties()).pump
}

func pcmBytesForDuration(duration time.Duration) int {
	return alignStereoFrame(int(int64(hostAudioSampleRate*4) * int64(duration) / int64(time.Second)))
}

func fmtAudioBufferPlan(plan audioBufferPlan) string {
	return fmt.Sprintf("queue_target=%s player_buffer=%s pump=%s", plan.queueTarget, plan.player, plan.pump)
}
