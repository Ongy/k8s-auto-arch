package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"golang.org/x/exp/slog"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func admissionReviewFromRequest(r *http.Request) (*admissionv1.AdmissionReview, error) {
	_, span := otel.Tracer("").Start(r.Context(), "admissionReviewFromRequest")
	defer span.End()

	// Validate that the incoming content type is correct.
	if r.Header.Get("Content-Type") != "application/json" {
		return nil, fmt.Errorf("expected application/json content-type")
	}

	if r.Body == nil {
		return nil, fmt.Errorf("request had empty body")
	}

	// Decode the request body into
	admissionReviewRequest := &admissionv1.AdmissionReview{}
	if err := json.NewDecoder(r.Body).Decode(&admissionReviewRequest); err != nil {
		return nil, fmt.Errorf("unmarshal admission request: %w", err)
	}

	return admissionReviewRequest, nil
}

func HandleRequest(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "Handling request", "client", r.RemoteAddr)
	// Parse the AdmissionReview from the http request.
	admissionReviewRequest, err := admissionReviewFromRequest(r)
	if err != nil {
		msg := fmt.Sprintf("error getting admission review from request: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// Do server-side validation that we are only dealing with a pod resource. This
	// should also be part of the MutatingWebhookConfiguration in the cluster, but
	// we should verify here before continuing.
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if admissionReviewRequest.Request.Resource != podResource {
		// While this isn't a big problem for the application, it's a warning level report of misconfiguration

		slog.ErrorContext(r.Context(), "Got incompatible resource", "client", r.RemoteAddr, "resource", admissionReviewRequest.Request.Resource)
		http.Error(w, "Incompatible resource", http.StatusBadRequest)
		return
	}

	admissionResponse, err := ReviewPod(r.Context(), admissionReviewRequest.Request)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to review resource", "err", err, "client", r.RemoteAddr)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Construct the response, which is just another AdmissionReview.
	var admissionReviewResponse admissionv1.AdmissionReview
	admissionReviewResponse.Response = admissionResponse
	admissionReviewResponse.SetGroupVersionKind(admissionReviewRequest.GroupVersionKind())
	admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID

	resp, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		slog.ErrorContext(r.Context(), "Error marshalling respons to json", "err", err)
		msg := fmt.Sprintf("error marshalling response json: %v", err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}
