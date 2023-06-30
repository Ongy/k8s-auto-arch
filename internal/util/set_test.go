package util

import (
	"testing"

	"golang.org/x/exp/maps"
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

func TestIntersect(t *testing.T) {
	testCases := []struct {
		name     string
		left     map[string]bool
		right    map[string]bool
		expected map[string]bool
	}{
		{
			name:     "empty",
			left:     map[string]bool{},
			right:    map[string]bool{},
			expected: map[string]bool{},
		},
		{
			name:     "lift-nil-empty",
			left:     nil,
			right:    map[string]bool{},
			expected: map[string]bool{},
		},
		{
			name:     "lift-nil-filled",
			left:     nil,
			right:    map[string]bool{"key": true},
			expected: map[string]bool{"key": true},
		},
		{
			name:     "equal-simple",
			left:     map[string]bool{"key": true},
			right:    map[string]bool{"key": true},
			expected: map[string]bool{"key": true},
		},
		{
			name:     "equal-complex",
			left:     map[string]bool{"key": true, "key2": false},
			right:    map[string]bool{"key": true, "key2": false},
			expected: map[string]bool{"key": true, "key2": false},
		},
		{
			name:     "missing-left",
			left:     map[string]bool{"key2": false},
			right:    map[string]bool{"key": true, "key2": false},
			expected: map[string]bool{"key2": false},
		},
		{
			name:     "missing-right",
			left:     map[string]bool{"key": true, "key2": false},
			right:    map[string]bool{"key2": false},
			expected: map[string]bool{"key2": false},
		},
		{
			name:     "right-biased",
			left:     map[string]bool{"key": true},
			right:    map[string]bool{"key": false},
			expected: map[string]bool{"key": false},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Intersect(testCase.left, testCase.right)

			if !maps.Equal(got, testCase.expected) {
				t.Errorf("got != wanted: %v != %v", got, testCase.expected)
			}
		})
	}
}
