package resources

import (
	"context"
	"fmt"
	"testing"

	"github.com/ongy/k8s-auto-arch/internal/resources/test"
	"github.com/ongy/k8s-auto-arch/internal/util"
	"golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
)

func TestContainerArchitectures(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		arches   map[string][]string
		expected []string
	}{
		{
			name:     "direct",
			input:    "registry.local/org/image",
			expected: []string{"amd64"},
		},
		{
			name:     "multi",
			input:    "registry.local/org/image",
			expected: []string{"amd64", "arm64"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			test.UseTestRegistry(map[test.ImageInfo][]string{{Organization: "org", Image: "image"}: testCase.expected})
			arches, err := containerArchitectures(context.Background(), testCase.input)
			if err != nil {
				t.Fatalf("Failed to get container architectures: %v", err)
			}

			want := testCase.expected
			got := util.Keys(arches)
			slices.Sort(want)
			slices.Sort(got)

			if !slices.Equal(got, want) {
				t.Errorf("got != want: %v != %v", got, want)
			}
		})
	}

}

func TestArchitectures(t *testing.T) {
	testCases := []struct {
		name     string
		input    v1.Pod
		arches   map[string][]string
		expected []string
	}{
		{
			name:     "empty",
			input:    v1.Pod{},
			arches:   map[string][]string{},
			expected: []string{},
		},
		{
			name: "simple",
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "image",
						},
					},
				},
			},
			arches: map[string][]string{
				"image": {"amd64"},
			},
			expected: []string{"amd64"},
		},
		{
			name: "multi-container",
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "image",
						},
						{
							Image: "image2",
						},
					},
				},
			},
			arches: map[string][]string{
				"image":  {"amd64"},
				"image2": {"amd64"},
			},
			expected: []string{"amd64"},
		},
		{
			name: "multi-arch",
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "image",
						},
						{
							Image: "image2",
						},
					},
				},
			},
			arches: map[string][]string{
				"image":  {"amd64", "arm64"},
				"image2": {"amd64", "arm64"},
			},
			expected: []string{"amd64", "arm64"},
		},
		{
			name: "multi-arch-intersect",
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "image",
						},
						{
							Image: "image2",
						},
					},
				},
			},
			arches: map[string][]string{
				"image":  {"amd64", "arm64"},
				"image2": {"amd64", "sparc"},
			},
			expected: []string{"amd64"},
		},
		{
			name: "multi-arch-intersect-init",
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "image",
						},
					},
					InitContainers: []v1.Container{
						{
							Image: "image2",
						},
					},
				},
			},
			arches: map[string][]string{
				"image":  {"amd64", "arm64"},
				"image2": {"amd64", "sparc"},
			},
			expected: []string{"amd64"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			doContainerArchitectures = func(_ context.Context, imgName string) (map[string]bool, error) {
				arches, ok := testCase.arches[imgName]
				if !ok {
					return nil, fmt.Errorf("couldn't find container")
				}

				ret := map[string]bool{}
				for _, arch := range arches {
					ret[arch] = true
				}

				return ret, nil
			}

			want := testCase.expected
			got, err := Architectures(context.Background(), &testCase.input)
			if err != nil {
				t.Errorf("Failed call to Architectures: %v", err)
				return
			}

			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("got != want: %v != %v", got, want)
			}
		})
	}

}
