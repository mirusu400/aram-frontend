package frontend

import "time"

// machineActivity is the coarse reading the status bar's signal indicator
// shows. It is deliberately coarser than BackendState: the indicator answers
// "is the guest running" at a glance, and the exact state is already spelled
// out beside it.
type machineActivity int

const (
	activityIdle machineActivity = iota
	activityPaused
	activityRunning
	activityFaulted
)

// batteryPollInterval is how often the host power state is re-read. Charge
// moves in percent-per-minute at best, and every reader behind readBattery is
// a system call or a sysfs read, so polling per frame would cost far more than
// the reading is worth.
const batteryPollInterval = 30 * time.Second

// machineActivity reports what the guest machine is doing.
func (s *Shell) machineActivity() machineActivity {
	switch s.backend.State() {
	case StateRunning:
		return activityRunning
	case StatePaused:
		return activityPaused
	case StateFaulted:
		return activityFaulted
	}
	return activityIdle
}

// hostBattery reports the host charge, re-reading it at most once per
// batteryPollInterval. Present is false where the platform exposes no battery,
// and the caller hides the indicator rather than showing a made-up charge.
func (s *Shell) hostBattery() batteryReading {
	now := time.Now()
	if s.batteryPolledAt.IsZero() || now.Sub(s.batteryPolledAt) >= batteryPollInterval {
		s.batteryPolledAt = now
		s.battery = readBattery()
	}
	return s.battery
}
