package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	cominTypes "github.com/nlewo/comin/internal/types"
	pb "github.com/nlewo/comin/pkg/protobuf"
	"github.com/stretchr/testify/require"
)

const fluxContentMediaType = "application/vnd.cncf.flux.content.v1.tar+gzip"

// newRegistry starts an in-memory registry and returns its host
func newRegistry(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(registry.New(registry.WithReferrersSupport(true), registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return u.Host
}

// tarGz returns a gzipped tar archive containing files
func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for n, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     n,
			Mode:     0644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// newArtifact returns an artifact looking like the ones produced by
// `flux push artifact`
func newArtifact(t *testing.T, files map[string]string, annotations map[string]string) v1.Image {
	t.Helper()
	img, err := mutate.AppendLayers(empty.Image, static.NewLayer(tarGz(t, files), fluxContentMediaType))
	require.NoError(t, err)
	img = mutate.MediaType(img, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, "application/vnd.cncf.flux.config.v1+json")
	return mutate.Annotations(img, annotations).(v1.Image)
}

// push pushes an artifact with the given created annotation and
// returns its digest
func push(t *testing.T, ref string, created string, files map[string]string) string {
	t.Helper()
	annotations := map[string]string{
		AnnotationRevision: "main@sha1:" + created,
		AnnotationSource:   "https://github.com/owner/infra",
	}
	if created != "" {
		annotations[AnnotationCreated] = created
	}
	img := newArtifact(t, files, annotations)
	r, err := name.ParseReference(ref, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(r, img))
	d, err := img.Digest()
	require.NoError(t, err)
	return d.String()
}

func newRemote(name, host string, main, testing cominTypes.OCIRef) cominTypes.OCIRemote {
	if main.Operation == "" {
		main.Operation = "switch"
	}
	return cominTypes.OCIRemote{
		Name:     name,
		URL:      "oci://" + host + "/owner/infra",
		Insecure: true,
		Timeout:  10,
		Ref: cominTypes.OCIRefs{
			Main:    main,
			Testing: testing,
		},
	}
}

func fetch(t *testing.T, r *repository, remoteNames ...string) *pb.OCIRepositoryStatus {
	t.Helper()
	return <-r.FetchAndUpdate(context.Background(), remoteNames)
}
