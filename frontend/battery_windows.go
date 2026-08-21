package frontend

import (
	"syscall"
	"unsafe"
)

// systemPowerStatus mirrors the Win32 SYSTEM_POWER_STATUS record.
type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

const (
	// batteryFlagNoBattery is set when the machine has no system battery at
	// all, which a desktop reports rather than reporting zero charge.
	batteryFlagNoBattery = 0x80
	// batteryPercentUnknown is what the API returns when it cannot read the
	// charge, including on a machine that is still enumerating its battery.
	batteryPercentUnknown = 255
	acLineOnline          = 1
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemPowerStatus = kernel32.NewProc("GetSystemPowerStatus")
)

func readBattery() batteryReading {
	var status systemPowerStatus
	ret, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 {
		return batteryReading{}
	}
	if status.BatteryFlag&batteryFlagNoBattery != 0 {
		return batteryReading{}
	}
	if status.BatteryLifePercent == batteryPercentUnknown {
		return batteryReading{}
	}
	return batteryReading{
		Percent:  int(status.BatteryLifePercent),
		Charging: status.ACLineStatus == acLineOnline,
		Present:  true,
	}
}
