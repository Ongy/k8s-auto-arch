package test

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

	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	registry "github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/exp/slices"
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

type ImageInfo struct {
	Organization string
	Image        string
}

func UseTestRegistry(images map[ImageInfo][]string) {
	files := []fileInfo{}
	for k, v := range images {
		files = append(files, makeInfos(v, k.Organization, k.Image)...)
	}

	registry.DefaultTransport = &testTripper{files}

}
