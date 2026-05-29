package util

import (
	"fmt"
	"os"
	"runtime"
	"slices"
)

func SortAndUniq(input []string) []string {
	if len(input) == 0 {
		return input
	}

	slices.Sort(input)

	result := []string{input[0]}
	for i := 1; i < len(input); i++ {
		if input[i] != input[i-1] {
			result = append(result, input[i])
		}
	}
	return result
}

func Sort(input []string) []string {
	slices.Sort(input)
	return input
}

var (
	sizeUnits = []string{"GB", "MB", "KB"}

	sizeMultipliers = map[string]int64{
		"GB": 1 << 20,
		"MB": 1 << 10,
		"KB": 1,
	}
)

// Format size in KBs to proper size units
func FormatSize(kbs int64) string {
	for _, unit := range sizeUnits {
		if multiplier := sizeMultipliers[unit]; kbs >= multiplier {
			value := float64(kbs) / float64(multiplier)
			if value == float64(int64(value)) {
				return fmt.Sprintf("%.0f%s", value, unit)
			} else {
				return fmt.Sprintf("%.1f%s", value, unit)
			}
		}
	}
	return "0"
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// CurrentPlatform returns the current system's platform
func CurrentPlatform() (string, string) {
	os := "macOS"
	if runtime.GOOS == "linux" {
		os = "Linux"
	}

	arch := "x86_64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}

	return os, arch
}
