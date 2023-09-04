package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"golang.org/x/exp/slog"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/ongy/k8s-auto-arch/internal/resources"
)

var (
	// Indirection for testing
	doArchitectures = resources.Architectures
	doHandlePod     = handlePod
)

func podAffinity(ctx context.Context, pod *corev1.Pod) (*corev1.Affinity, error) {
	ctx, span := otel.Tracer("").Start(ctx, "podAffinity")
	defer span.End()

	podArches, err := doArchitectures(ctx, pod)
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

func handlePod(ctx context.Context, pod *corev1.Pod) (string, error) {
	ctx, span := otel.Tracer("").Start(ctx, "handlePod")
	defer span.End()

	if pod.Spec.Affinity == nil {
		affinity, err := podAffinity(ctx, pod)
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

func ReviewPod(ctx context.Context, request *v1.AdmissionRequest) (*admissionv1.AdmissionResponse, error) {
	ctx, span := otel.Tracer("").Start(ctx, "ReviewPod")
	defer span.End()

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

	patch, err := doHandlePod(ctx, &pod)
	if err != nil {
		return nil, fmt.Errorf("get pod patch: %w", err)
	}

	admissionResponse.Allowed = true
	if patch != "" {
		admissionResponse.PatchType = &patchType
		admissionResponse.Patch = []byte(patch)
		slog.InfoContext(ctx, "Annotating pod")
	} else {
		slog.DebugContext(ctx, "Skipping pod")
	}

	return admissionResponse, nil
}
