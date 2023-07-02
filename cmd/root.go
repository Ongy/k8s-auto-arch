package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ongy/k8s-auto-arch/internal/controller"
	"github.com/spf13/cobra"

	goflags "flag"

	"k8s.io/klog/v2"
)

var (
	gitDescribe string

	port int
)

var rootCmd = &cobra.Command{
	Use:   "k8s-auto-arch",
	Short: "Kubernetes auto architecture assignment",
	Long: `Small webhook service that allows to automatically inject architecture node affinity.
	
	This is useful when when you have a mixed architecture cluster but cannot guarantee that every container is available in all arches.
	When this is used as mutating webhook, it will automatically download the container manifests and check for compatible platforms.
	
	When the pod already has an affinity configured, it's not updated.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return runWebhookServer(ctx)
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		klog.V(3).InfoS("Starting k8s-auto-arch", "version", gitDescribe)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	fs := goflags.NewFlagSet("", goflags.PanicOnError)
	klog.InitFlags(fs)
	rootCmd.PersistentFlags().AddGoFlagSet(fs)

	rootCmd.Flags().IntVar(&port, "port", 8080, "Port to listen on for HTTPS traffic")
}

func runWebhookServer(ctx context.Context) error {
	http.HandleFunc("/", controller.HandleRequest)
	server := http.Server{
		Addr:     fmt.Sprintf(":%d", port),
		ErrorLog: klog.NewStandardLogger("ERROR"),
	}

	go func() {
		<-ctx.Done()

		if err := server.Shutdown(context.Background()); err != nil {
			klog.V(2).InfoS("Failed to shutdown server", "err", err, "port", port)
		}
	}()

	if err := server.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("ListenAndServe: %w", err)
		}
	}

	return nil
}
