package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	regname "github.com/google/go-containerregistry/pkg/name"
	registry "github.com/google/go-containerregistry/pkg/v1/remote"
)

var (
	tlsCert string
	tlsKey  string
	port    int
	codecs  = serializer.NewCodecFactory(runtime.NewScheme())
	logger  = log.New(os.Stdout, "http: ", log.LstdFlags)
)

var rootCmd = &cobra.Command{
	Use:   "mutating-webhook",
	Short: "Kubernetes mutating webhook example",
	Long: `Example showing how to implement a basic mutating webhook in Kubernetes.

Example:
$ mutating-webhook --tls-cert <tls_cert> --tls-key <tls_key> --port <port>`,
	Run: func(cmd *cobra.Command, args []string) {
		//if tlsCert == "" || tlsKey == "" {
		//	fmt.Println("--tls-cert and --tls-key required")
		//	os.Exit(1)
		//}

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		runWebhookServer(ctx, tlsCert, tlsKey)
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

func admissionReviewFromRequest(r *http.Request, deserializer runtime.Decoder) (*admissionv1.AdmissionReview, error) {
	// Validate that the incoming content type is correct.
	if r.Header.Get("Content-Type") != "application/json" {
		return nil, fmt.Errorf("expected application/json content-type")
	}

	// Get the body data, which will be the AdmissionReview
	// content for the request.
	var body []byte
	if r.Body != nil {
		requestData, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		body = requestData
	}

	// Decode the request body into
	admissionReviewRequest := &admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, admissionReviewRequest); err != nil {
		return nil, err
	}

	return admissionReviewRequest, nil
}

func containerArchitectures(refString string) ([]string, error) {
	ref, err := regname.ParseReference(refString)
	if err != nil {
		return []string{}, fmt.Errorf("parse image reference: %w", err)
	}
	index, err := registry.Index(ref)
	if err != nil {
		image, err := registry.Image(ref)
		if err != nil {
			return []string{}, fmt.Errorf("get image: %w", err)
		}

		imageConfig, err := image.ConfigFile()
		if err != nil {
			return []string{}, fmt.Errorf("get imageConfig: %w", err)
		}

		return []string{imageConfig.Architecture}, nil
	}

	manifest, err := index.IndexManifest()
	if err != nil {
		return []string{}, fmt.Errorf("get index manifest: %w", err)
	}

	//TODO: Solve for OS as well!
	aggregator := []string{}
	for _, image := range manifest.Manifests {
		//aggregator[image.Platform.Architecture] = true
		aggregator = append(aggregator, image.Platform.Architecture)
	}

	return aggregator, nil
}

func mutatePod(w http.ResponseWriter, r *http.Request) {
	logger.Printf("received message on mutate")

	deserializer := codecs.UniversalDeserializer()

	// Parse the AdmissionReview from the http request.
	admissionReviewRequest, err := admissionReviewFromRequest(r, deserializer)
	if err != nil {
		msg := fmt.Sprintf("error getting admission review from request: %v", err)
		logger.Printf(msg)
		w.WriteHeader(400)
		w.Write([]byte(msg))
		return
	}

	// Do server-side validation that we are only dealing with a pod resource. This
	// should also be part of the MutatingWebhookConfiguration in the cluster, but
	// we should verify here before continuing.
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if admissionReviewRequest.Request.Resource != podResource {
		msg := fmt.Sprintf("did not receive pod, got %s", admissionReviewRequest.Request.Resource.Resource)
		logger.Printf(msg)
		w.WriteHeader(400)
		w.Write([]byte(msg))
		return
	}

	// Decode the pod from the AdmissionReview.
	rawRequest := admissionReviewRequest.Request.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := deserializer.Decode(rawRequest, nil, &pod); err != nil {
		msg := fmt.Sprintf("error decoding raw pod: %v", err)
		logger.Printf(msg)
		w.WriteHeader(500)
		w.Write([]byte(msg))
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

		for _, container := range pod.Spec.Containers {
			arches, err := containerArchitectures(container.Image)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get contianer images for %v: %v\n", container.Name, err)
			} else {
				fmt.Printf("Looking at container: %v: %v\n", container.Name, arches)
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
										Values:   arches,
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
			fmt.Printf("Patching with: %s\n", patch)
		}
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
		w.WriteHeader(500)
		w.Write([]byte(msg))
		return
	}

	fmt.Printf("%s\n", resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

func runWebhookServer(ctx context.Context, certFile, keyFile string) {
	//cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	//if err != nil {
	//	panic(err)
	//}

	fmt.Println("Starting webhook server")
	http.HandleFunc("/", mutatePod)
	server := http.Server{
		Addr: fmt.Sprintf(":%d", port),
		//		TLSConfig: &tls.Config{
		//			Certificates: []tls.Certificate{cert},
		//		},
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
			fmt.Fprintf(os.Stderr, "Server closed with error: %v\n", err)
		}
	}
}
