package main

import (
	"sort"
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
