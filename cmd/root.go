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
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	regname "github.com/google/go-containerregistry/pkg/name"
	registry "github.com/google/go-containerregistry/pkg/v1/remote"

	util "github.com/ongy/k8s-auto-arch/internal/util"
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

func containerArchitectures(refString string) (map[string]bool, error) {
	ref, err := regname.ParseReference(refString)
	if err != nil {
		return map[string]bool{}, fmt.Errorf("parse image reference: %w", err)
	}
	index, err := registry.Index(ref)
	if err != nil {
		image, err := registry.Image(ref)
		if err != nil {
			return map[string]bool{}, fmt.Errorf("get image: %w", err)
		}

		imageConfig, err := image.ConfigFile()
		if err != nil {
			return map[string]bool{}, fmt.Errorf("get imageConfig: %w", err)
		}

		return map[string]bool{imageConfig.Architecture: true}, nil
	}

	manifest, err := index.IndexManifest()
	if err != nil {
		return map[string]bool{}, fmt.Errorf("get index manifest: %w", err)
	}

	//TODO: Solve for OS as well!
	aggregator := map[string]bool{}
	for _, image := range manifest.Manifests {
		//aggregator[image.Platform.Architecture] = true
		aggregator[image.Platform.Architecture] = true
	}

	return aggregator, nil
}

func podArchitectures(pod *corev1.Pod) ([]string, error) {
	var podArches map[string]bool
	for _, container := range pod.Spec.Containers {
		arches, err := containerArchitectures(container.Image)
		if err != nil {
			return []string{}, fmt.Errorf("get arches of container '%s': %w", container.Name, err)
		}

		podArches = util.Intersect(podArches, arches)
	}

	for _, container := range pod.Spec.InitContainers {
		arches, err := containerArchitectures(container.Image)
		if err != nil {
			return []string{}, fmt.Errorf("get arches of container '%s': %w", container.Name, err)
		}

		podArches = util.Intersect(podArches, arches)
	}

	return util.Keys(podArches), nil
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

	// Decode the pod from the AdmissionReview.
	rawRequest := admissionReviewRequest.Request.Object.Raw
	pod := corev1.Pod{}
	if err := json.Unmarshal(rawRequest, &pod); err != nil {
		msg := fmt.Sprintf("error decoding raw pod: %v", err)
		logger.Printf(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	// Create a response that will add a label to the pod if it does
	// not already have a label with the key of "hello". In this case
	// it does not matter what the value is, as long as the key exists.
	admissionResponse := &admissionv1.AdmissionResponse{}
	var patch string
	patchType := v1.PatchTypeJSONPatch
	if _, ok := pod.Labels["hello"]; !ok {
		patch = `[{"op":"add","path":"/metadata/labels/hello","value":"world"}]`
	}

	if pod.Spec.Affinity == nil {
		podArches, err := podArchitectures(&pod)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get pod architectures: %v\n", err)
			http.Error(w, fmt.Errorf("pod architectures: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		affinity := corev1.Affinity{
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
		}

		affinityStr, err := json.Marshal(map[string]interface{}{"op": "add", "path": "/spec/affinity", "value": affinity})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal affinity: %v\n", err)
		}
		patch = fmt.Sprintf(`[%s]`, affinityStr)
	} else {
		fmt.Printf("Skipping pod with pre-set affinity!\n")
	}

	admissionResponse.Allowed = true
	if patch != "" {
		admissionResponse.PatchType = &patchType
		admissionResponse.Patch = []byte(patch)
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
