package frontend

import (
	"testing"
	"time"
)

func TestHapticMagnitude(t *testing.T) {
	cases := []struct {
		name    string
		state   HapticsState
		wantMag float64
		wantOn  bool
	}{
		{name: "idle", state: HapticsState{}, wantMag: 0, wantOn: false},
		{
			name:    "level without duration",
			state:   HapticsState{Level: 80},
			wantMag: 0, wantOn: false,
		},
		{
			name:    "half strength",
			state:   HapticsState{Level: 50, Duration: 200 * time.Millisecond},
			wantMag: 0.5, wantOn: true,
		},
		{
			name:    "full strength",
			state:   HapticsState{Level: 100, Duration: time.Second},
			wantMag: 1, wantOn: true,
		},
		{
			name:    "over 100 clamps to 1",
			state:   HapticsState{Level: 200, Duration: time.Second},
			wantMag: 1, wantOn: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mag, on := hapticMagnitude(tc.state)
			if on != tc.wantOn || mag != tc.wantMag {
				t.Fatalf("hapticMagnitude(%+v) = (%v,%v), want (%v,%v)",
					tc.state, mag, on, tc.wantMag, tc.wantOn)
			}
		})
	}
}

func TestStartsHapticPulse(t *testing.T) {
	if !startsHapticPulse(false, 0, 100*time.Millisecond) {
		t.Fatal("an inactive-to-active request did not start a phone pulse")
	}
	if startsHapticPulse(true, 100*time.Millisecond, 84*time.Millisecond) {
		t.Fatal("a continuing request restarted the phone pulse")
	}
	if !startsHapticPulse(true, 20*time.Millisecond, 100*time.Millisecond) {
		t.Fatal("a back-to-back request with a renewed duration was missed")
	}
}

func TestHapticPreviewPulse(t *testing.T) {
	magnitude, duration, ok := hapticPreviewPulse(true)
	if !ok {
		t.Fatal("switching vibration on must confirm with a pulse")
	}
	if magnitude <= 0 || magnitude > 1 {
		t.Fatalf("preview magnitude %v out of 0..1", magnitude)
	}
	if duration <= 0 {
		t.Fatalf("preview duration %v must be positive", duration)
	}
	if _, _, ok := hapticPreviewPulse(false); ok {
		t.Fatal("switching vibration off must not buzz")
	}
}

func TestButtonHapticPulseIsShortAndTouchOnly(t *testing.T) {
	magnitude, duration, ok := buttonHapticPulse(true, true)
	if !ok || magnitude != buttonHapticMagnitude || duration != buttonHapticDuration {
		t.Fatalf("button pulse = (%v, %v, %v)", magnitude, duration, ok)
	}
	if duration >= hapticPreviewDuration {
		t.Fatal("button feedback should be subtler than the settings preview")
	}
	for _, test := range []struct {
		touch, enabled bool
	}{
		{touch: false, enabled: true},
		{touch: true, enabled: false},
	} {
		if _, _, ok := buttonHapticPulse(test.touch, test.enabled); ok {
			t.Fatalf("button pulse unexpectedly enabled for %+v", test)
		}
	}
}
