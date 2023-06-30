package util

import (
	"testing"

	"golang.org/x/exp/slices"
)

func TestKeys(t *testing.T) {
	testCases := []struct {
		name     string
		input    map[string]any
		expected []string
	}{
		{
			name:     "empty",
			input:    map[string]any{},
			expected: []string{},
		},
		{
			name:     "simple",
			input:    map[string]any{"one": "value"},
			expected: []string{"one"},
		},
		{
			name:     "multi",
			input:    map[string]any{"one": "value", "two": "value", "three": false},
			expected: []string{"one", "three", "two"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Keys(testCase.input)
			want := slices.Clone(testCase.expected)

			slices.Sort(got)
			slices.Sort(want)

			if !slices.Equal(got, want) {
				t.Errorf("got != wanted: %v != %v", got, want)
			}
		})
	}
}
