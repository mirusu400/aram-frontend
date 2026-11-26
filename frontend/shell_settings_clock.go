package frontend

import (
	"fmt"
	"time"
)

// The handset clock is what a title reads as the current date and time. It
// follows this device's clock by default. A custom time is stored as an offset
// from the device clock, the way a real handset's date setting behaves: once
// set, it keeps running, so a title that measures time away from the game
// still sees time pass between sessions.

// Handset calendars of the period stop short of 2038, where a signed 32-bit
// count of seconds overflows, and titles were not written for dates before
// their own platform existed.
const (
	handsetClockMinYear = 2000
	handsetClockMaxYear = 2037
)

type handsetClockField int

const (
	handsetClockYear handsetClockField = iota
	handsetClockMonth
	handsetClockDay
	handsetClockHour
	handsetClockMinute
)

// handsetClockNow is the device clock; tests replace it.
var handsetClockNow = time.Now

// handsetClockOffset is how far the handset clock is set from the device
// clock. It is zero while the clock follows the device.
func (s *Shell) handsetClockOffset() time.Duration {
	if !s.settings.HandsetClockCustom {
		return 0
	}
	return time.Duration(s.settings.HandsetClockOffsetSeconds) * time.Second
}

// handsetClockTime is the date and time a title opened now would read, in the
// device's time zone.
func (s *Shell) handsetClockTime() time.Time {
	return handsetClockNow().Add(s.handsetClockOffset())
}

func (s *Shell) currentClockSettings() ClockSettings {
	return ClockSettings{Offset: s.handsetClockOffset()}
}

func (s *Shell) handsetClockModeLabel(custom bool) string {
	if custom {
		return s.tr("Custom time")
	}
	return s.tr("This device's time")
}

// setHandsetClockCustom switches between following the device clock and a
// custom time. Turning custom off forgets the offset, so turning it back on
// starts from the device's time again.
func (s *Shell) setHandsetClockCustom(custom bool) {
	if s.settings.HandsetClockCustom == custom {
		return
	}
	s.settings.HandsetClockCustom = custom
	s.settings.HandsetClockOffsetSeconds = 0
	s.applyClockSettings()
}

// handsetClockFieldRange reports the first value and the number of values one
// date field offers for the handset clock's current date.
func (s *Shell) handsetClockFieldRange(field handsetClockField) (first, count int) {
	switch field {
	case handsetClockYear:
		return handsetClockMinYear, handsetClockMaxYear - handsetClockMinYear + 1
	case handsetClockMonth:
		return 1, 12
	case handsetClockDay:
		current := s.handsetClockTime()
		return 1, daysInMonth(current.Year(), current.Month())
	case handsetClockHour:
		return 0, 24
	default:
		return 0, 60
	}
}

func (s *Shell) handsetClockFieldValue(field handsetClockField) int {
	current := s.handsetClockTime()
	switch field {
	case handsetClockYear:
		return clampInt(current.Year(), handsetClockMinYear, handsetClockMaxYear)
	case handsetClockMonth:
		return int(current.Month())
	case handsetClockDay:
		return current.Day()
	case handsetClockHour:
		return current.Hour()
	default:
		return current.Minute()
	}
}

func (s *Shell) handsetClockFieldLabel(field handsetClockField, value int) string {
	if field == handsetClockYear {
		return fmt.Sprintf("%04d", value)
	}
	return fmt.Sprintf("%02d", value)
}

// setHandsetClockField sets one field of the handset clock's date and keeps
// the others, pulling the day back when the new month is shorter.
func (s *Shell) setHandsetClockField(field handsetClockField, value int) {
	now := handsetClockNow()
	current := now.Add(s.handsetClockOffset())
	year, month, day := current.Date()
	hour, minute := current.Hour(), current.Minute()
	switch field {
	case handsetClockYear:
		year = value
	case handsetClockMonth:
		month = time.Month(clampInt(value, 1, 12))
	case handsetClockDay:
		day = value
	case handsetClockHour:
		hour = clampInt(value, 0, 23)
	default:
		minute = clampInt(value, 0, 59)
	}
	year = clampInt(year, handsetClockMinYear, handsetClockMaxYear)
	day = clampInt(day, 1, daysInMonth(year, month))
	target := time.Date(
		year, month, day, hour, minute,
		current.Second(), current.Nanosecond(), current.Location(),
	)
	s.settings.HandsetClockCustom = true
	s.settings.HandsetClockOffsetSeconds = int64(target.Sub(now).Round(time.Second) / time.Second)
	s.applyClockSettings()
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// handsetClockSignature changes whenever the clock rows must be rebuilt: when
// the custom rows appear or disappear, and when the month length changes.
func (s *Shell) handsetClockSignature() string {
	if !s.settings.HandsetClockCustom {
		return "device"
	}
	_, days := s.handsetClockFieldRange(handsetClockDay)
	return fmt.Sprintf("custom/%d", days)
}

// applyClockSettings persists the selection and pushes it to the backend. The
// new time takes effect the next time a title is opened.
func (s *Shell) applyClockSettings() {
	_ = s.settings.save()
	if backend, ok := s.backend.(ClockBackend); ok {
		if err := backend.ConfigureClock(s.currentClockSettings()); err != nil {
			s.setStatus(s.tr("Handset clock: ") + err.Error())
			return
		}
	}
	s.setStatus(s.trf(
		"Handset clock: %s (restart title to apply)",
		s.handsetClockTime().Format("2006-01-02 15:04"),
	))
}
