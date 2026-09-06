package frontend

import "math"

// audioSpeed is the guest-to-real time ratio the output stretches PCM by.
//
// The guest renders a fixed amount of audio per guest frame. When the pacer
// runs the guest at anything other than real time - a speed setting, or the
// few percent display sync accepts to land frames on refreshes - the host
// queue is fed at that ratio and drifts into a steady underrun or overrun.
// Re-labelling each chunk's sample rate by the ratio makes the existing host
// resampler stretch it to real time instead, with the pitch shift that
// implies: a 0.96x synchronized title plays 4% low, a 2x fast-forward plays an
// octave up rather than dropping every other block.
type audioSpeed struct {
	ratio float64
}

// setSpeed records the ratio. Zero and one both mean no stretching.
func (o *audioOutput) setSpeed(ratio float64) {
	if o == nil {
		return
	}
	if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		ratio = 1
	}
	o.speed.ratio = ratio
}

// stretchedSampleRate is the rate a chunk should be encoded as so that
// playback covers real time. It leaves the rate alone when there is nothing
// to stretch or when the result would fall outside what the encoder accepts,
// so an extreme ratio degrades to the old queue behaviour rather than an
// error on every chunk.
func (o *audioOutput) stretchedSampleRate(sampleRate int) int {
	if o == nil || o.speed.ratio <= 0 || o.speed.ratio == 1 || sampleRate <= 0 {
		return sampleRate
	}
	stretched := int(math.Round(float64(sampleRate) * o.speed.ratio))
	if stretched < hostAudioMinSampleRate || stretched > hostAudioMaxSampleRate {
		return sampleRate
	}
	return stretched
}
