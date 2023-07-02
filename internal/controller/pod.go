package controller

import (
	"encoding/json"
	"fmt"
	"os"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/ongy/k8s-auto-arch/internal/resources"
)

var (
	// Indirection for testing
	doArchitectures = resources.Architectures
	doHandlePod     = handlePod
)

func podAffinity(pod *corev1.Pod) (*corev1.Affinity, error) {
	podArches, err := doArchitectures(pod)
	if err != nil {
		return nil, fmt.Errorf("get pod architectures: %w", err)
	}

	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/arch",
								Operator: "In",
								Values:   podArches,
							},
						},
					},
				},
			},
		},
	}, nil
}

func handlePod(pod *corev1.Pod) (string, error) {
	if pod.Spec.Affinity == nil {
		affinity, err := podAffinity(pod)
		if err != nil {
			return "", fmt.Errorf("get pod affinity: %w", err)
		}

		affinityStr, err := json.Marshal(map[string]interface{}{"op": "add", "path": "/spec/affinity", "value": affinity})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal affinity: %v\n", err)
		}
		return fmt.Sprintf(`[%s]`, affinityStr), nil
	}

	return "", nil
}

func ReviewPod(request *v1.AdmissionRequest) (*admissionv1.AdmissionResponse, error) {
	rawRequest := request.Object.Raw
	pod := corev1.Pod{}
	if err := json.Unmarshal(rawRequest, &pod); err != nil {
		return nil, fmt.Errorf("decode raw pod: %w", err)
	}

	// Create a response that will add a label to the pod if it does
	// not already have a label with the key of "hello". In this case
	// it does not matter what the value is, as long as the key exists.
	admissionResponse := &admissionv1.AdmissionResponse{}
	patchType := v1.PatchTypeJSONPatch

	patch, err := doHandlePod(&pod)
	if err != nil {
		return nil, fmt.Errorf("get pod patch: %w", err)
	}

	admissionResponse.Allowed = true
	if patch != "" {
		admissionResponse.PatchType = &patchType
		admissionResponse.Patch = []byte(patch)
		klog.V(3).InfoS("Annotating pod", "pod", pod.Name)
	} else {
		klog.V(4).InfoS("Skipping pod", "pod", pod.Name)
	}

	return admissionResponse, nil
}
