package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/ongy/k8s-auto-arch/internal/resources/test"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

func makeAdmissionRequest(pod *v1.Pod) admissionv1.AdmissionReview {
	raw, _ := json.Marshal(pod)

	return admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Resource: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object: runtime.RawExtension{
				Raw: raw,
			},
		},
	}
}

func makeHttpRequest(pod *v1.Pod) http.Request {
	requestURL, _ := url.Parse("http://127.0.0.1/")
	body, _ := json.Marshal(makeAdmissionRequest(pod))

	return http.Request{
		Method: "POST",
		URL:    requestURL,
		Body:   io.NopCloser(bytes.NewReader(body)),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}
}

func handleResponse(res *http.Response, pod *v1.Pod) (*v1.Pod, error) {
	var admissinoReview admissionv1.AdmissionReview
	err := json.NewDecoder(res.Body).Decode(&admissinoReview)
	if err != nil {
		return nil, fmt.Errorf("decode review: %w", err)
	}

	if admissinoReview.Response.PatchType == nil {
		return pod, nil
	}

	patch, err := jsonpatch.DecodePatch(admissinoReview.Response.Patch)
	if err != nil {
		return nil, fmt.Errorf("decode patch: %w", err)
	}

	podJSON, _ := json.Marshal(pod)
	patchedJSON, err := patch.Apply(podJSON)
	if err != nil {
		return nil, fmt.Errorf("apply patch: %w", err)
	}

	var ret v1.Pod
	if err := json.Unmarshal(patchedJSON, &ret); err != nil {
		return nil, fmt.Errorf("decode patched json: %w", err)
	}

	return &ret, nil
}

func TestHandleRequest(t *testing.T) {
	testCases := []struct {
		name     string
		arches   map[test.ImageInfo][]string
		input    v1.Pod
		expected v1.Pod
	}{
		{
			name:   "simple",
			arches: map[test.ImageInfo][]string{test.ImageInfo{"org", "image"}: {"amd64"}},
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "registry.local/org/image:latest",
						},
					},
				},
			},
			expected: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "registry.local/org/image:latest",
						},
					},
					Affinity: &v1.Affinity{
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
			},
		},
		{
			name:   "skip",
			arches: map[test.ImageInfo][]string{test.ImageInfo{"org", "image"}: {"amd64"}},
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "registry.local/org/image:latest",
						},
					},
					Affinity: &v1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/other",
												Operator: "In",
												Values:   []string{"sample"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "registry.local/org/image:latest",
						},
					},
					Affinity: &v1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/other",
												Operator: "In",
												Values:   []string{"sample"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:   "intersect",
			arches: map[test.ImageInfo][]string{{Organization: "org", Image: "image"}: {"amd64"}, {Organization: "org", Image: "image2"}: {"amd64", "arm64"}},
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "registry.local/org/image:latest",
						},
						{
							Image: "registry.local/org/image2:latest",
						},
					},
				},
			},
			expected: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "registry.local/org/image:latest",
						},
						{
							Image: "registry.local/org/image2:latest",
						},
					},
					Affinity: &v1.Affinity{
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
			},
		},
		{
			name:   "multi",
			arches: map[test.ImageInfo][]string{{Organization: "org", Image: "image"}: {"amd64", "arm64"}, {Organization: "org", Image: "image2"}: {"amd64", "arm64"}},
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "registry.local/org/image:latest",
						},
						{
							Image: "registry.local/org/image2:latest",
						},
					},
				},
			},
			expected: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "registry.local/org/image:latest",
						},
						{
							Image: "registry.local/org/image2:latest",
						},
					},
					Affinity: &v1.Affinity{
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
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			test.UseTestRegistry(testCase.arches)
			recorder := httptest.NewRecorder()
			request := makeHttpRequest(&testCase.input)

			HandleRequest(recorder, &request)

			res := recorder.Result()
			if res.StatusCode != http.StatusOK {
				t.Fatalf("Got non-200 handler return: %v", res.StatusCode)
			}

			got, err := handleResponse(res, &testCase.input)
			if err != nil {
				t.Fatalf("Failed to handle response: %v", err)
			}

			if !reflect.DeepEqual(got, &testCase.expected) {
				t.Errorf("got != want: %v != %v", got, testCase.expected)
			}
		})
	}

}
