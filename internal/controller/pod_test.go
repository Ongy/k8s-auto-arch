package controller

import (
	"encoding/json"
	"reflect"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

func TestPodAffinity(t *testing.T) {
	testCases := []struct {
		name     string
		arches   []string
		input    v1.Pod
		expected v1.Affinity
	}{
		{
			name:   "simple",
			arches: []string{"amd64"},
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "doesn't matter",
						},
					},
				},
			},
			expected: corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/arch",
										Operator: "In",
										Values:   []string{"amd64"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:   "multi",
			arches: []string{"amd64", "arm64"},
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "doesn't matter",
						},
					},
				},
			},
			expected: corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/arch",
										Operator: "In",
										Values:   []string{"amd64", "arm64"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			doArchitectures = func(pod *v1.Pod) ([]string, error) {
				return testCase.arches, nil
			}

			got, err := podAffinity(&testCase.input)
			if err != nil {
				t.Fatalf("Get pod affinity: %v", err)
			}

			if !reflect.DeepEqual(got, &testCase.expected) {
				t.Errorf("got != wanted: %v != %v", got, &testCase.expected)
			}
		})
	}
}

func TestHandlePod(t *testing.T) {
	testCases := []struct {
		name     string
		arches   []string
		input    v1.Pod
		expected *corev1.Affinity
	}{
		{
			name:   "simple",
			arches: []string{"amd64"},
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "doesn't matter",
						},
					},
				},
			},
			expected: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/arch",
										Operator: "In",
										Values:   []string{"amd64"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:   "pre-existing",
			arches: []string{"amd64"},
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "doesn't matter",
						},
					},
					Affinity: &v1.Affinity{},
				},
			},
			expected: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			doArchitectures = func(pod *v1.Pod) ([]string, error) {
				return testCase.arches, nil
			}

			got, err := handlePod(&testCase.input)
			if err != nil {
				t.Fatalf("Get pod affinity: %v", err)
			}
			if testCase.expected == nil {
				if got != "" {
					t.Fatalf("Expected no patch, but got patch :()")
				}
				return
			}

			var unmarshalled []map[string]any
			if err := json.Unmarshal([]byte(got), &unmarshalled); err != nil {
				t.Fatalf("Failed to unmarshall the json: %v", err)
			}

			if len(unmarshalled) != 1 {
				t.Fatalf("Got unexpect length != 1: %d", len(unmarshalled))
			}

			if _, ok := unmarshalled[0]["value"]; !ok {
				t.Fatalf("Path string is missing the value")
			}

			affinityStr, err := json.Marshal(unmarshalled[0]["value"])
			if err != nil {
				t.Fatalf("Failed to marshal affinity: %v", err)
			}

			var affinity v1.Affinity
			if err := json.Unmarshal(affinityStr, &affinity); err != nil {
				t.Fatalf("Failed to unmarshal affinity: %v", err)
			}

			if !reflect.DeepEqual(&affinity, testCase.expected) {
				t.Errorf("got != wanted: %v != %v", got, &testCase.expected)
			}
		})
	}
}

func TestReviewPod(t *testing.T) {
	patchType := admissionv1.PatchTypeJSONPatch
	testCases := []struct {
		name     string
		patch    string
		expected *admissionv1.AdmissionResponse
	}{
		{
			name:  "nopatch",
			patch: "",
			expected: &admissionv1.AdmissionResponse{
				Allowed: true,
			},
		},
		{
			name:  "patch",
			patch: "patch",
			expected: &admissionv1.AdmissionResponse{
				Patch:     []byte("patch"),
				PatchType: &patchType,
				Allowed:   true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			doHandlePod = func(pod *v1.Pod) (string, error) { return testCase.patch, nil }
			request := admissionv1.AdmissionRequest{}

			request.Object.Raw = []byte("{}")

			got, err := ReviewPod(&request)
			if err != nil {
				t.Fatalf("Failed to review pod: %v", err)
			}

			if !reflect.DeepEqual(got, testCase.expected) {
				t.Fatalf("got != want: %v != %v", got, testCase.expected)
			}
		})
	}
}
