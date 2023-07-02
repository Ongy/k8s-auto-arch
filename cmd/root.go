package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
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

func runWebhookServer(ctx context.Context, certFile, keyFile string) error {

	fmt.Println("Starting webhook server")
	http.HandleFunc("/", controller.HandleRequest)
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
