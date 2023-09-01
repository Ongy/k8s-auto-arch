package resources

import (
	"context"
	"fmt"

	regname "github.com/google/go-containerregistry/pkg/name"
	registry "github.com/google/go-containerregistry/pkg/v1/remote"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"

	"github.com/ongy/k8s-auto-arch/internal/util"
)

var (
	// Indirection for testing!
	doContainerArchitectures = containerArchitectures
)

func containerArchitectures(ctx context.Context, refString string) (map[string]bool, error) {
	ctx, span := otel.Tracer("").Start(ctx, "containerArchitectures", trace.WithAttributes(attribute.String("container", refString)))
	defer span.End()

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
		aggregator[image.Platform.Architecture] = true
	}

	return aggregator, nil
}

func Architectures(ctx context.Context, pod *corev1.Pod) ([]string, error) {
	ctx, span := otel.Tracer("").Start(ctx, "Architectures")
	defer span.End()

	var podArches map[string]bool
	for _, container := range pod.Spec.Containers {
		arches, err := doContainerArchitectures(ctx, container.Image)
		if err != nil {
			return []string{}, fmt.Errorf("get arches of container '%s': %w", container.Name, err)
		}

		podArches = util.Intersect(podArches, arches)
	}

	for _, container := range pod.Spec.InitContainers {
		arches, err := doContainerArchitectures(ctx, container.Image)
		if err != nil {
			return []string{}, fmt.Errorf("get arches of initContainer '%s': %w", container.Name, err)
		}

		podArches = util.Intersect(podArches, arches)
	}

	return util.Keys(podArches), nil
}
