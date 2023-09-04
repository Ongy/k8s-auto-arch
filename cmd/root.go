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

	"github.com/ongy/k8s-auto-arch/internal/controller"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	metricexporter "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/spf13/cobra"

	"golang.org/x/exp/slog"
)

const serviceName = "ks-auto-arch.ongy.net"

var (
	gitDescribe  string
	collectorURL = ""

	port int
)

func initTracer() func(context.Context) error {
	if collectorURL == "" {
		return func(context.Context) error { return nil }
	}

	exporter, err := otlptrace.New(
		context.Background(),
		otlptracegrpc.NewClient(
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(collectorURL),
		),
	)

	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}
	resources, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("library.language", "go"),
		),
	)
	if err != nil {
		log.Fatalf("Could not set resources: %v", err)
	}

	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(resources),
		),
	)
	return exporter.Shutdown
}

func initMeter(ctx context.Context) (*sdkmetric.MeterProvider, error) {
	exp, err := metricexporter.New(ctx, metricexporter.WithInsecure(), metricexporter.WithEndpoint(collectorURL))
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)))
	otel.SetMeterProvider(mp)
	return mp, nil
}

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

		if _, err := initMeter(ctx); err != nil {
			return fmt.Errorf("initMeter: %w", err)
		}

		stopTracer := initTracer()
		defer stopTracer(ctx)

		return runWebhookServer(ctx)
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true})
		slog.SetDefault(slog.New(handler))

		slog.InfoContext(cmd.Context(), "Starting k8s-auto-arch", "version", gitDescribe)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.Flags().IntVar(&port, "port", 8080, "Port to listen on for HTTPS traffic")
	rootCmd.PersistentFlags().StringVar(&collectorURL, "otlp_collector", "", "Set the open telemetry collector URI")
}

func runWebhookServer(ctx context.Context) error {
	http.Handle("/", otelhttp.NewHandler(http.HandlerFunc(controller.HandleRequest), "root"))
	server := http.Server{
		Addr:     fmt.Sprintf(":%d", port),
		ErrorLog: slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	}

	go func() {
		<-ctx.Done()

		if err := server.Shutdown(context.Background()); err != nil {
			slog.ErrorContext(ctx, "Failed to shutdown server", "err", err, "port", port)
		}
	}()

	if err := server.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("ListenAndServe: %w", err)
		}
	}

	return nil
}
