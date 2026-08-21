package frontend

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// powerSupplyRoot is the kernel's power supply class, which Android exposes
// the same way desktop Linux does. Reading it needs no permission and no
// platform bridge; where a policy blocks it the read simply fails and the
// indicator stays hidden.
const powerSupplyRoot = "/sys/class/power_supply"

func readBattery() batteryReading {
	entries, err := os.ReadDir(powerSupplyRoot)
	if err != nil {
		return batteryReading{}
	}
	for _, entry := range entries {
		supply := filepath.Join(powerSupplyRoot, entry.Name())
		if readPowerSupplyField(supply, "type") != "Battery" {
			continue
		}
		percent, err := strconv.Atoi(readPowerSupplyField(supply, "capacity"))
		if err != nil || percent < 0 || percent > 100 {
			continue
		}
		return batteryReading{
			Percent:  percent,
			Charging: readPowerSupplyField(supply, "status") == "Charging",
			Present:  true,
		}
	}
	return batteryReading{}
}

func readPowerSupplyField(supply, field string) string {
	raw, err := os.ReadFile(filepath.Join(supply, field))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
