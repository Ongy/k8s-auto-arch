package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ongy/k8s-auto-arch/internal/controller"
)

var (
	gitDescribe string

	tlsCert string
	tlsKey  string
	port    int
	logger  = log.New(os.Stdout, "http: ", log.LstdFlags)
)

var rootCmd = &cobra.Command{
	Use:   "mutating-webhook",
	Short: "Kubernetes mutating webhook example",
	Long: `Example showing how to implement a basic mutating webhook in Kubernetes.

Example:
$ mutating-webhook --tls-cert <tls_cert> --tls-key <tls_key> --port <port>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return runWebhookServer(ctx, tlsCert, tlsKey)
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Running k8s-auto-arch version %s\n", gitDescribe)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.Flags().StringVar(&tlsCert, "tls-cert", "", "Certificate for TLS")
	rootCmd.Flags().StringVar(&tlsKey, "tls-key", "", "Private key file for TLS")
	rootCmd.Flags().IntVar(&port, "port", 8080, "Port to listen on for HTTPS traffic")
}

func admissionReviewFromRequest(r *http.Request) (*admissionv1.AdmissionReview, error) {
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

func mutatePod(w http.ResponseWriter, r *http.Request) {
	logger.Printf("received message on mutate")

	// Parse the AdmissionReview from the http request.
	admissionReviewRequest, err := admissionReviewFromRequest(r)
	if err != nil {
		msg := fmt.Sprintf("error getting admission review from request: %v", err)
		logger.Printf(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// Do server-side validation that we are only dealing with a pod resource. This
	// should also be part of the MutatingWebhookConfiguration in the cluster, but
	// we should verify here before continuing.
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if admissionReviewRequest.Request.Resource != podResource {
		msg := fmt.Sprintf("did not receive pod, got %s", admissionReviewRequest.Request.Resource.Resource)
		logger.Printf(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	admissionResponse, err := controller.ReviewPod(admissionReviewRequest.Request)
	if err != nil {
		logger.Printf("Get admission response: %v", err)
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
		msg := fmt.Sprintf("error marshalling response json: %v", err)
		logger.Printf(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

func runWebhookServer(ctx context.Context, certFile, keyFile string) error {

	fmt.Println("Starting webhook server")
	http.HandleFunc("/", mutatePod)
	server := http.Server{
		Addr:     fmt.Sprintf(":%d", port),
		ErrorLog: logger,
	}

	go func() {
		<-ctx.Done()

		if err := server.Shutdown(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to shutdown server: %v\n", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("ListenAndServe: %w", err)
		}
	}

	return nil
}
