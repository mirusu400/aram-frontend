//go:build !windows && !linux

package frontend

// readBattery reports no battery on platforms whose power state this build has
// no verified way to read. The status bar hides the indicator rather than
// showing an invented charge.
func readBattery() batteryReading { return batteryReading{} }
