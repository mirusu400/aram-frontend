package frontend

import (
	"strings"
	"testing"
	"time"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
)

func indicatorShell(t *testing.T) *Shell {
	t.Helper()
	shell := NewShell(NullBackend{}, nil, "")
	shell.settings.ThemeMode = "dark"
	shell.settings.ThemeFamily = "candy-orange"
	shell.syncDesignSystem()
	return shell
}

func visible(w *widget.Widget) bool {
	return w.GetVisibility() == widget.Visibility_Show
}

// The charge meter reports the host battery or nothing at all. A shell running
// where no battery can be read must not draw a full one.
func TestChargeMeterHiddenWithoutARealReading(t *testing.T) {
	shell := indicatorShell(t)
	view := shell.interfaceUI
	if view.statusBattery == nil {
		t.Fatal("sprite skin built no battery indicator")
	}
	shell.battery = batteryReading{}
	shell.batteryPolledAt = time.Now()
	view.syncStatusIndicators(shell, 1000)
	if visible(view.statusBattery.GetWidget()) {
		t.Fatal("battery icon shown with no reading behind it")
	}
	if view.statusBatteryText.Label != "" {
		t.Fatalf("battery label = %q with no reading", view.statusBatteryText.Label)
	}

	shell.battery = batteryReading{Percent: 62, Present: true}
	shell.batteryPolledAt = time.Now()
	view.syncStatusIndicators(shell, 1000)
	if !visible(view.statusBattery.GetWidget()) {
		t.Fatal("battery icon hidden with a reading present")
	}
	if !strings.Contains(view.statusBatteryText.Label, "62") {
		t.Fatalf("battery label = %q, want the measured charge", view.statusBatteryText.Label)
	}
}

// A low battery has to be distinguishable from a healthy one; the pack ships
// one glyph, so the reading is carried by the ink.
func TestChargeMeterInksLowAndChargingApart(t *testing.T) {
	shell := indicatorShell(t)
	view := shell.interfaceUI
	readings := []batteryReading{
		{Percent: 80, Present: true},
		{Percent: batteryLow, Present: true},
		{Percent: 80, Present: true, Charging: true},
	}
	seen := make(map[*ebiten.Image]bool)
	for _, reading := range readings {
		shell.battery = reading
		shell.batteryPolledAt = time.Now()
		view.syncStatusIndicators(shell, 1000)
		seen[view.statusBattery.Image] = true
	}
	if len(seen) != len(readings) {
		t.Fatalf("charge readings collapsed onto %d glyphs, want %d",
			len(seen), len(readings))
	}
}

// The signal glyph reports what the guest machine is doing, so a running
// machine and an idle one cannot look the same.
func TestSignalGlyphSeparatesRunningFromIdle(t *testing.T) {
	shell := indicatorShell(t)
	view := shell.interfaceUI
	if view.statusSignal == nil {
		t.Fatal("sprite skin built no signal indicator")
	}
	if got := shell.machineActivity(); got != activityIdle {
		t.Fatalf("fresh shell activity = %v, want idle", got)
	}
	view.syncStatusIndicators(shell, 1000)
	idle := view.statusSignal.Image

	running := &stateBackend{state: StateRunning}
	shell.backend = running
	if got := shell.machineActivity(); got != activityRunning {
		t.Fatalf("running backend activity = %v", got)
	}
	view.syncStatusIndicators(shell, 1000)
	if view.statusSignal.Image == idle {
		t.Fatal("running and idle machines drew the same signal glyph")
	}
}

// The cluster is the first thing to give way when the bar runs out of room.
func TestIndicatorsStandDownOnANarrowBar(t *testing.T) {
	shell := indicatorShell(t)
	view := shell.interfaceUI
	shell.battery = batteryReading{Percent: 50, Present: true}
	shell.batteryPolledAt = time.Now()
	view.syncStatusIndicators(shell, 320)
	if visible(view.statusSignal.GetWidget()) ||
		visible(view.statusBattery.GetWidget()) {
		t.Fatal("indicator cluster kept its room on a narrow bar")
	}
}

// stateBackend is a NullBackend that reports a state of the test's choosing.
type stateBackend struct {
	NullBackend
	state BackendState
}

func (b *stateBackend) State() BackendState { return b.state }
