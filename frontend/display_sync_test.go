package frontend

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

// runSyncedPacing drives ticks the way Update does - counting the host tick
// first, then pacing - and returns how many quanta each tick issued.
func runSyncedPacing(
	shell *Shell,
	advance func(time.Duration),
	quantum, tick, total time.Duration,
) []int {
	var issued []int
	for elapsed := time.Duration(0); elapsed < total; elapsed += tick {
		advance(tick)
		shell.recordHostTick(shell.now())
		shell.accumulateFramePacing(shell.now(), quantum)
		issued = append(issued, shell.takeFrameQuanta(quantum))
	}
	return issued
}

func TestDisplaySyncPlanLandsFramesOnRefreshes(t *testing.T) {
	const wipi = 16 * time.Millisecond
	tests := []struct {
		name     string
		tickRate float64
		quantum  time.Duration
		speed    float64
		enabled  bool
		budget   time.Duration
		achieved float64
		ok       bool
	}{
		{"60Hz WIPI", 60, wipi, 1, true, wipi, 0.96, true},
		{"120Hz WIPI", 120, wipi, 1, true, wipi / 2, 0.96, true},
		{"240Hz WIPI", 240, wipi, 1, true, wipi / 4, 0.96, true},
		{"60Hz KTF", 60, ktfQuantum, 1, true, ktfQuantum, 1, true},
		{"60Hz WIPI at 2x", 60, wipi, 2, true, 2 * wipi, 1.92, true},
		{"60Hz WIPI at 0.5x", 60, wipi, 0.5, true, wipi / 2, 0.48, true},
		{"144Hz is too far", 144, wipi, 1, true, 0, 0, false},
		{"50Hz is too far", 50, wipi, 1, true, 0, 0, false},
		{"75Hz is too far", 75, wipi, 1, true, 0, 0, false},
		{"disabled", 60, wipi, 1, false, 0, 0, false},
		{"unmeasured", 0, wipi, 1, true, 0, 0, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			budget, achieved, ok := displaySyncPlan(
				test.tickRate, test.quantum, test.speed, test.enabled,
			)
			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}
			if !ok {
				return
			}
			if budget != test.budget {
				t.Fatalf("budget = %v, want %v", budget, test.budget)
			}
			if math.Abs(achieved-test.achieved) > 1e-6 {
				t.Fatalf("achieved = %v, want %v", achieved, test.achieved)
			}
		})
	}
}

// TestDisplaySyncIssuesExactlyOneQuantumPerRefresh is the stutter this exists
// to remove: 62.5 guest frames against 60 ticks used to mean a tick carrying
// two quanta every 400 ms. Once the tick rate is known, every tick carries
// exactly one.
func TestDisplaySyncIssuesExactlyOneQuantumPerRefresh(t *testing.T) {
	const quantum = 16 * time.Millisecond
	const tick = time.Second / 60
	shell, advance := newPacingShell(quantum)
	shell.settings.DisplaySync = true
	issued := runSyncedPacing(shell, advance, quantum, tick, 3*time.Second)
	// The first window measures the rate; everything after it must be steady.
	for i, count := range issued[len(issued)/3:] {
		if count != 1 {
			t.Fatalf("tick %d issued %d quanta under display sync", i, count)
		}
	}
	if !shell.displaySync.active {
		t.Fatal("display sync never engaged")
	}
	if got := shell.effectivePacingSpeed(); math.Abs(got-0.96) > 1e-6 {
		t.Fatalf("effective speed = %v, want 0.96", got)
	}
	if got := shell.audioSpeed; math.Abs(got-0.96) > 1e-6 {
		t.Fatalf("audio speed = %v, want 0.96", got)
	}
}

func TestDisplaySyncFallsBackToRealTime(t *testing.T) {
	const quantum = 16 * time.Millisecond
	tests := []struct {
		name  string
		tick  time.Duration
		setup func(*Shell)
	}{
		{"without vsync", time.Second / 60, func(s *Shell) { s.settings.VsyncDisabled = true }},
		{"when disabled", time.Second / 60, func(s *Shell) { s.settings.DisplaySync = false }},
		{"on a 144Hz display", time.Second / 144, func(s *Shell) {}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shell, advance := newPacingShell(quantum)
			shell.settings.DisplaySync = true
			test.setup(shell)
			const total = 10 * time.Second
			issued := runSyncedPacing(shell, advance, quantum, test.tick, total)
			var advanced time.Duration
			for _, count := range issued {
				advanced += time.Duration(count) * quantum
			}
			if drift := (advanced - total).Abs(); drift > quantum {
				t.Fatalf("guest advanced %v over %v real", advanced, total)
			}
			if shell.displaySync.active {
				t.Fatal("display sync engaged where it should not")
			}
			if got := shell.effectivePacingSpeed(); got != 1 {
				t.Fatalf("effective speed = %v, want 1", got)
			}
		})
	}
}

// A late tick under display sync is a dropped refresh, not a debt to work
// off with a burst: the guest carries on one frame per tick.
func TestDisplaySyncDoesNotBurstAfterAStall(t *testing.T) {
	const quantum = 16 * time.Millisecond
	const tick = time.Second / 60
	shell, advance := newPacingShell(quantum)
	shell.settings.DisplaySync = true
	runSyncedPacing(shell, advance, quantum, tick, 2*time.Second)
	advance(200 * time.Millisecond)
	shell.recordHostTick(shell.now())
	shell.accumulateFramePacing(shell.now(), quantum)
	if got := shell.takeFrameQuanta(quantum); got != 1 {
		t.Fatalf("stalled tick issued %d quanta, want 1", got)
	}
}

func TestHostTickRateIsMeasuredOverAWindow(t *testing.T) {
	shell, advance := newPacingShell(16 * time.Millisecond)
	shell.recordHostTick(shell.now())
	for range 30 {
		advance(time.Second / 60)
		shell.recordHostTick(shell.now())
	}
	if shell.displaySync.tickRate != 0 {
		t.Fatalf("rate reported before the window closed: %v", shell.displaySync.tickRate)
	}
	for range 31 {
		advance(time.Second / 60)
		shell.recordHostTick(shell.now())
	}
	if got := shell.displaySync.tickRate; math.Abs(got-60) > 0.5 {
		t.Fatalf("tick rate = %v, want ~60", got)
	}
	// A stall inside the next window is excised rather than read as a
	// slower display.
	advance(300 * time.Millisecond)
	shell.recordHostTick(shell.now())
	for range 61 {
		advance(time.Second / 60)
		shell.recordHostTick(shell.now())
	}
	if got := shell.displaySync.tickRate; math.Abs(got-60) > 0.5 {
		t.Fatalf("tick rate after a stall = %v, want ~60", got)
	}
}

func TestSpeedReadoutNamesTheSynchronizedRate(t *testing.T) {
	shell, _ := newPacingShell(16 * time.Millisecond)
	shell.displaySync = displaySyncState{active: true, tickRate: 60, speed: 0.96}
	shell.measuredSpeed = 0.96
	if got := shell.speedSettingValue(); got != "1x/60Hz (100%)" {
		t.Fatalf("readout = %q", got)
	}
	shell.displaySync.active = false
	if got := shell.speedSettingValue(); got != "1x (96%)" {
		t.Fatalf("readout without sync = %q", got)
	}
	report := shell.debugPacingReport()
	if report.EffectiveSpeed != 1 || report.DisplaySyncActive {
		t.Fatalf("report = %+v", report)
	}
}

func TestDisplaySyncDefaultsOnAndPersistsOff(t *testing.T) {
	defaults := defaultSettings()
	if !defaults.DisplaySync {
		t.Fatal("display sync should default on")
	}
	if defaults.VsyncDisabled {
		t.Fatal("vsync should default on")
	}
	blob, err := json.Marshal(Settings{DisplaySync: false})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(blob), `"display_sync":false`) {
		t.Fatalf("display_sync was omitted from %s", blob)
	}
	loaded := defaultSettings()
	if err := json.Unmarshal(blob, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.DisplaySync {
		t.Fatal("an explicit off did not survive a load over the on default")
	}
	// A settings file written before the option existed keeps the default.
	legacy := defaultSettings()
	if err := json.Unmarshal([]byte(`{"speed":1}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if !legacy.DisplaySync {
		t.Fatal("a legacy file lost the display-sync default")
	}
}

func TestDisplaySyncAndVsyncRowsInExperiments(t *testing.T) {
	isolateSettings(t)
	shell := NewShell(NullBackend{}, nil, "")
	shell.toggleDisplaySync()
	if shell.settings.DisplaySync {
		t.Fatal("toggle did not disable display sync")
	}
	shell.toggleVsync()
	if !shell.settings.VsyncDisabled {
		t.Fatal("toggle did not disable vsync")
	}
	shell.toggleVsync()
	if shell.settings.VsyncDisabled {
		t.Fatal("toggle did not re-enable vsync")
	}
	u := shell.interfaceUI
	u.settingsSection = "Experiments"
	found := map[string]bool{}
	for _, row := range u.settingsRowModels(shell) {
		if row.action != nil {
			found[row.label] = true
		}
	}
	for _, label := range []string{"Display sync", "Vsync", "UI priority"} {
		if !found[label] {
			t.Fatalf("Experiments section is missing the %s row", label)
		}
	}
}

func TestAudioOutputStretchesSampleRateBySpeed(t *testing.T) {
	output := &audioOutput{}
	if got := output.stretchedSampleRate(44_100); got != 44_100 {
		t.Fatalf("unset speed changed the rate to %d", got)
	}
	output.setSpeed(0.96)
	if got := output.stretchedSampleRate(44_100); got != 42_336 {
		t.Fatalf("0.96x rate = %d, want 42336", got)
	}
	output.setSpeed(2)
	if got := output.stretchedSampleRate(44_100); got != 88_200 {
		t.Fatalf("2x rate = %d, want 88200", got)
	}
	// A stretch the encoder could not accept is dropped rather than erroring
	// on every chunk.
	output.setSpeed(0.5)
	if got := output.stretchedSampleRate(8_000); got != 8_000 {
		t.Fatalf("out-of-range stretch produced %d", got)
	}
	output.setSpeed(math.NaN())
	if got := output.stretchedSampleRate(44_100); got != 44_100 {
		t.Fatalf("NaN speed changed the rate to %d", got)
	}

	// End to end: 960 guest frames at 0.96x must become 1000 host frames -
	// the real time they cover - rather than 960.
	const latency = 60 * time.Millisecond
	output = &audioOutput{
		queue:          newPCMQueue(audioQueueBytes(latency)),
		prebufferBytes: audioPrebufferBytes(latency),
		prebufferWait:  latency,
	}
	output.setSpeed(0.96)
	if err := output.enqueue(AudioChunk{
		SampleRate: hostAudioSampleRate,
		Channels:   2,
		PCM16:      make([]int16, 960*2),
	}, 0, 0); err != nil {
		t.Fatal(err)
	}
	if got := output.queue.availableBytes(); got != 1000*4 {
		t.Fatalf("queued %d bytes, want %d", got, 1000*4)
	}
}
