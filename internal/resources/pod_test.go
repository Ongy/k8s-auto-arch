package resources

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	registry "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/ongy/k8s-auto-arch/internal/util"
	"golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
)

var (
	firstManifest = []byte(`{
   "schemaVersion": 1,
   "name": "radicale",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:b65dff7e1796c4dac1873afc70d2ce8faf866016e8218b0a49200a0fb7a8d350"
      },
      {
         "blobSum": "sha256:c8f3bc40d6ea2ab1ff86011a14860c4cdf5169e12fa7fb9001f58214571699df"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:63b65145d645c1250c391b2d16ebe53b3747c295ca8ba2fcb6b0cf064a4dc21c"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"architecture\":\"amd64\",\"config\":{\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"PYTHONUNBUFFERED=1\"],\"Entrypoint\":[\"/usr/bin/radicale\"],\"WorkingDir\":\"/\",\"OnBuild\":null},\"created\":\"2023-04-19T09:03:58.935540517+02:00\",\"id\":\"e7a347ad0b9b178f13a6a141dbcdf99ccbeb8ba2e6fb80a764407ef2dfed7e71\",\"moby.buildkit.buildinfo.v1\":\"eyJmcm9udGVuZCI6ImRvY2tlcmZpbGUudjAiLCJhdHRycyI6eyJmaWxlbmFtZSI6IkRvY2tlcmZpbGUucmFkaWNhbGUifSwic291cmNlcyI6W3sidHlwZSI6ImRvY2tlci1pbWFnZSIsInJlZiI6ImRvY2tlci5pby9saWJyYXJ5L2FscGluZTpsYXRlc3QifSx7InR5cGUiOiJkb2NrZXItaW1hZ2UiLCJyZWYiOiJkb2NrZXIuaW8vbGlicmFyeS9hbHBpbmU6bGF0ZXN0IiwicGluIjoic2hhMjU2OjdjZDUyODQ3YWQ3NzVhNWRkYzRiNTgzMjZjZjg4NGJlZWUzNDU0NDI5NjQwMmM2MjkyZWQ3NjQ3NGM2ODZkMzkifV19\",\"os\":\"linux\",\"parent\":\"4d01e569b588c8b57c2243d3a13fdafd775f296bf2b8010406704def7e5d8840\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"4d01e569b588c8b57c2243d3a13fdafd775f296bf2b8010406704def7e5d8840\",\"parent\":\"7e0328c12b1ceb232eaaeea1ae8b9123648d80b661fd655d8bd8c33641f658fc\",\"comment\":\"buildkit.dockerfile.v0\",\"created\":\"2023-04-19T09:03:58.935540517+02:00\",\"container_config\":{\"Cmd\":[\"WORKDIR /\"]},\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"7e0328c12b1ceb232eaaeea1ae8b9123648d80b661fd655d8bd8c33641f658fc\",\"parent\":\"dd87a16ea23ab6f17fff223a8aa08ccb00f1846b493d7b0f1685a8e8076840a4\",\"comment\":\"buildkit.dockerfile.v0\",\"created\":\"2023-04-19T09:03:58.935540517+02:00\",\"container_config\":{\"Cmd\":[\"RUN /bin/sh -c python3 -m ensurepip \\u0026\\u0026 pip3 install --no-cache --upgrade radicale # buildkit\"]}}"
      },
      {
         "v1Compatibility": "{\"id\":\"dd87a16ea23ab6f17fff223a8aa08ccb00f1846b493d7b0f1685a8e8076840a4\",\"parent\":\"79421282ee6f483433bb1ddf700367e25ad547321237d02790aefe5a263aecc1\",\"comment\":\"buildkit.dockerfile.v0\",\"created\":\"2023-04-19T09:03:51.066438359+02:00\",\"container_config\":{\"Cmd\":[\"RUN /bin/sh -c apk add --update --no-cache python3 \\u0026\\u0026 ln -sf python3 /usr/bin/python # buildkit\"]}}"
      },
      {
         "v1Compatibility": "{\"id\":\"79421282ee6f483433bb1ddf700367e25ad547321237d02790aefe5a263aecc1\",\"parent\":\"b8e19f728db100f0501b117fa12192906fe642596ca0ca0487b40ddef2c01ffb\",\"comment\":\"buildkit.dockerfile.v0\",\"created\":\"2023-04-19T09:03:51.066438359+02:00\",\"container_config\":{\"Cmd\":[\"ENV PYTHONUNBUFFERED=1\"]},\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"b8e19f728db100f0501b117fa12192906fe642596ca0ca0487b40ddef2c01ffb\",\"parent\":\"d9035ba329737e7478c3cf1d1b69690dde0c0439c30d82622ded46ac9b7c212f\",\"created\":\"2023-02-11T04:46:42.558343068Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop)  CMD [\\\"/bin/sh\\\"]\"]},\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"d9035ba329737e7478c3cf1d1b69690dde0c0439c30d82622ded46ac9b7c212f\",\"created\":\"2023-02-11T04:46:42.449083344Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) ADD file:40887ab7c06977737e63c215c9bd297c0c74de8d12d16ebdf1c3d40ac392f62d in / \"]}}"
      }
   ],
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "NULW:DTQZ:4DGV:Q23S:YDE4:YPQK:OZUB:PZ4Q:H2K4:JE6O:AFOY:K5M5",
               "kty": "EC",
               "x": "S4-OujtMG954eQYx6JMAKSngm5q4zR9sv0PTmHb2b5Y",
               "y": "4zNwkpbfqLPtmeY8m_BnFFB_DRBehjw9Ymq8MG3MykQ"
            },
            "alg": "ES256"
         },
         "signature": "vaZ-FZHAC1PujbGpfaOzcqIjVAJb6F2yKqCjH0nQn9dTok2xv2K9_oxQjKcHEsOEXzKZjJE3p6JsDNqsIy4O4g",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjQxMjMsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAyMy0wNy0wMVQxODozNDozOFoifQ"
      }
   ]
}`)
	configFile = []byte(`{"architecture":"amd64","config":{"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin","PYTHONUNBUFFERED=1"],"Entrypoint":["/usr/bin/radicale"],"WorkingDir":"/","OnBuild":null},"created":"2023-04-19T09:03:58.935540517+02:00","history":[{"created":"2023-02-11T04:46:42.449083344Z","created_by":"/bin/sh -c #(nop) ADD file:40887ab7c06977737e63c215c9bd297c0c74de8d12d16ebdf1c3d40ac392f62d in / "},{"created":"2023-02-11T04:46:42.558343068Z","created_by":"/bin/sh -c #(nop)  CMD [\"/bin/sh\"]","empty_layer":true},{"created":"2023-04-19T09:03:51.066438359+02:00","created_by":"ENV PYTHONUNBUFFERED=1","comment":"buildkit.dockerfile.v0","empty_layer":true},{"created":"2023-04-19T09:03:51.066438359+02:00","created_by":"RUN /bin/sh -c apk add --update --no-cache python3 \u0026\u0026 ln -sf python3 /usr/bin/python # buildkit","comment":"buildkit.dockerfile.v0"},{"created":"2023-04-19T09:03:58.935540517+02:00","created_by":"RUN /bin/sh -c python3 -m ensurepip \u0026\u0026 pip3 install --no-cache --upgrade radicale # buildkit","comment":"buildkit.dockerfile.v0"},{"created":"2023-04-19T09:03:58.935540517+02:00","created_by":"WORKDIR /","comment":"buildkit.dockerfile.v0","empty_layer":true},{"created":"2023-04-19T09:03:58.935540517+02:00","created_by":"ENTRYPOINT [\"/usr/bin/radicale\"]","comment":"buildkit.dockerfile.v0","empty_layer":true}],"moby.buildkit.buildinfo.v1":"eyJmcm9udGVuZCI6ImRvY2tlcmZpbGUudjAiLCJhdHRycyI6eyJmaWxlbmFtZSI6IkRvY2tlcmZpbGUucmFkaWNhbGUifSwic291cmNlcyI6W3sidHlwZSI6ImRvY2tlci1pbWFnZSIsInJlZiI6ImRvY2tlci5pby9saWJyYXJ5L2FscGluZTpsYXRlc3QifSx7InR5cGUiOiJkb2NrZXItaW1hZ2UiLCJyZWYiOiJkb2NrZXIuaW8vbGlicmFyeS9hbHBpbmU6bGF0ZXN0IiwicGluIjoic2hhMjU2OjdjZDUyODQ3YWQ3NzVhNWRkYzRiNTgzMjZjZjg4NGJlZWUzNDU0NDI5NjQwMmM2MjkyZWQ3NjQ3NGM2ODZkMzkifV19","os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:7cd52847ad775a5ddc4b58326cf884beee34544296402c6292ed76474c686d39","sha256:fd389f3de3f9ba4f223e98c46e8c235fd5dd1b304d604da756569e15ae714a78","sha256:049c3bf18ff4305edb25e5c6f4a13b51525de623b7a0809d2a930d2c126c78e5"]}}`)
)

type fileInfo struct {
	path        string
	content     []byte
	contentType string
}

func makeManifestFile(path string, config fileInfo) fileInfo {
	manifest := registryv1.Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
		Config: registryv1.Descriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Size:      int64(len(config.content)),
			Digest:    registryv1.Hash{Algorithm: "sha256", Hex: config.path},
		},
	}

	content, _ := json.Marshal(manifest)

	return fileInfo{
		path:        path,
		content:     content,
		contentType: "application/vnd.docker.distribution.manifest.v2+json",
	}
}

func makeArchConfig(architecture string) fileInfo {
	content := []byte(fmt.Sprintf(`{"architecture":%q}`, architecture))

	hasher := sha256.New()
	hasher.Write(content)

	hash := hasher.Sum([]byte{})
	dst := make([]byte, len(hash)*2)
	hex.Encode(dst, hash)

	return fileInfo{
		path:        string(dst),
		content:     content,
		contentType: "application/vnd.docker.container.image.v1+json",
	}
}

func makeSingleInfos(architecture, org, image string) []fileInfo {
	config := makeArchConfig(architecture)
	manifest := makeManifestFile(fmt.Sprintf("/v2/%s/%s/manifests/latest", org, image), config)

	return []fileInfo{config, manifest}
}

func makeInfos(architectures []string, org, image string) []fileInfo {
	if len(architectures) == 1 {
		return makeSingleInfos(architectures[0], org, image)
	}

	manifests := []registryv1.Descriptor{}
	for _, arch := range architectures {
		manifests = append(manifests, registryv1.Descriptor{
			Digest: registryv1.Hash{
				Algorithm: "sha256",
				Hex:       "0000000000000000000000000000000000000000000000000000000000000000",
			},
			MediaType: "application/vnd.oci.image.manifest.v1+json",
			Platform: &registryv1.Platform{
				Architecture: arch,
			},
		})
	}
	manifest := registryv1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.index.v1+json",
		Manifests:     manifests,
	}

	content, _ := json.Marshal(manifest)
	return []fileInfo{
		{
			path:        fmt.Sprintf("/v2/%s/%s/manifests/latest", org, image),
			content:     content,
			contentType: "application/vnd.oci.image.index.v1+json",
		},
	}
}

type testTripper struct {
	files []fileInfo
}

func (t *testTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() == "http://registry.local/v2/" {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("{}")),
		}, nil
	}

	for _, file := range t.files {
		if req.URL.Path == file.path || strings.HasSuffix(req.URL.Path, fmt.Sprintf("/sha256:%s", file.path)) {
			if accept, ok := req.Header["Accept"]; ok {
				if !slices.Contains(strings.Split(accept[0], ","), file.contentType) {
					continue
				}
			}

			return &http.Response{
				StatusCode:    http.StatusOK,
				Body:          io.NopCloser(bytes.NewBuffer(file.content)),
				ContentLength: int64(len(file.content)),
				Header: http.Header{
					"Content-Type": []string{file.contentType},
				}}, nil
		}
	}

	return nil, errors.New("Not implemented yet")
}

func TestContainerArchitectures(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		arches   map[string][]string
		expected []string
	}{
		{
			name:     "direct",
			input:    "registry.local/org/image",
			expected: []string{"amd64"},
		},
		{
			name:     "multi",
			input:    "registry.local/org/image",
			expected: []string{"amd64", "arm64"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			registry.DefaultTransport = &testTripper{makeInfos(testCase.expected, "org", "image")}
			arches, err := containerArchitectures(testCase.input)
			if err != nil {
				t.Fatalf("Failed to get container architectures: %v", err)
			}

			want := testCase.expected
			got := util.Keys(arches)
			slices.Sort(want)
			slices.Sort(got)

			if !slices.Equal(got, want) {
				t.Errorf("got != want: %v != %v", got, want)
			}
		})
	}

}

func TestArchitectures(t *testing.T) {
	testCases := []struct {
		name     string
		input    v1.Pod
		arches   map[string][]string
		expected []string
	}{
		{
			name:     "empty",
			input:    v1.Pod{},
			arches:   map[string][]string{},
			expected: []string{},
		},
		{
			name: "simple",
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "image",
						},
					},
				},
			},
			arches: map[string][]string{
				"image": {"amd64"},
			},
			expected: []string{"amd64"},
		},
		{
			name: "multi-container",
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "image",
						},
						{
							Image: "image2",
						},
					},
				},
			},
			arches: map[string][]string{
				"image":  {"amd64"},
				"image2": {"amd64"},
			},
			expected: []string{"amd64"},
		},
		{
			name: "multi-arch",
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "image",
						},
						{
							Image: "image2",
						},
					},
				},
			},
			arches: map[string][]string{
				"image":  {"amd64", "arm64"},
				"image2": {"amd64", "arm64"},
			},
			expected: []string{"amd64", "arm64"},
		},
		{
			name: "multi-arch-intersect",
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "image",
						},
						{
							Image: "image2",
						},
					},
				},
			},
			arches: map[string][]string{
				"image":  {"amd64", "arm64"},
				"image2": {"amd64", "sparc"},
			},
			expected: []string{"amd64"},
		},
		{
			name: "multi-arch-intersect-init",
			input: v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "image",
						},
					},
					InitContainers: []v1.Container{
						{
							Image: "image2",
						},
					},
				},
			},
			arches: map[string][]string{
				"image":  {"amd64", "arm64"},
				"image2": {"amd64", "sparc"},
			},
			expected: []string{"amd64"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			doContainerArchitectures = func(imgName string) (map[string]bool, error) {
				arches, ok := testCase.arches[imgName]
				if !ok {
					return nil, fmt.Errorf("couldn't find container")
				}

				ret := map[string]bool{}
				for _, arch := range arches {
					ret[arch] = true
				}

				return ret, nil
			}

			want := testCase.expected
			got, err := Architectures(&testCase.input)
			if err != nil {
				t.Errorf("Failed call to Architectures: %v", err)
				return
			}

			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("got != want: %v != %v", got, want)
			}
		})
	}

}
