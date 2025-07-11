package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
)

func sortAndUniq(input []string) []string {
	if len(input) == 0 {
		return input
	}

	sort.Slice(input, func(i, j int) bool {
		return input[i] < input[j]
	})

	result := []string{input[0]}
	for i := 1; i < len(input); i++ {
		if input[i] != input[i-1] {
			result = append(result, input[i])
		}
	}
	return result
}

var (
	sizeUnits = []string{"GB", "MB", "KB", "B"}

	sizeMultipliers = map[string]int64{
		"GB": 1 << 30,
		"MB": 1 << 20,
		"KB": 1 << 10,
		"B":  1,
	}
)

func parseSizeToBytes(s string) int64 {
	if s == "" {
		return 0
	}

	for _, unit := range sizeUnits {
		if strings.HasSuffix(s, unit) {
			value, err := strconv.ParseFloat(strings.TrimSuffix(s, unit), 64)
			if err != nil {
				log.Printf("Error parsing size %s: %+v", s, err)
				return 0
			}
			size := value * float64(sizeMultipliers[unit])
			return int64(size)
		}
	}

	return 0
}

func formatSize(bytes int64) string {
	for _, unit := range sizeUnits {
		if multiplier := sizeMultipliers[unit]; bytes >= multiplier {
			value := float64(bytes) / float64(multiplier)
			if value == float64(int64(value)) {
				return fmt.Sprintf("%.0f%s", value, unit)
			} else {
				return fmt.Sprintf("%.1f%s", value, unit)
			}
		}
	}
	return "0"
}
