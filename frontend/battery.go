package frontend

// batteryReading is the host power state behind the status bar's battery
// indicator. Present is false wherever the platform has no battery or does not
// expose one; the indicator is then hidden rather than guessed at, because a
// charge meter that is not measuring anything is worse than no meter.
type batteryReading struct {
	Percent  int
	Charging bool
	Present  bool
}

// batteryLow is the charge below which the indicator switches to the fault
// ink. It matches the point handset shells of the era started warning.
const batteryLow = 15
