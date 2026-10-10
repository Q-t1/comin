package oci

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	cominTypes "github.com/nlewo/comin/internal/types"
	"github.com/sigstore/sigstore-go/pkg/sign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

const bundleMediaType = "application/vnd.dev.sigstore.bundle.v0.3+json"

// newKeypair returns a keypair and the path of its PEM public key
func newKeypair(t *testing.T) (*sign.EphemeralKeypair, string) {
	t.Helper()
	kp, err := sign.NewEphemeralKeypair(nil)
	require.NoError(t, err)
	pem, err := kp.GetPublicKeyPem()
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "cosign.pub")
	require.NoError(t, os.WriteFile(path, []byte(pem), 0600))
	return kp, path
}

func subject(t *testing.T, ref string) (name.Digest, v1.Descriptor) {
	t.Helper()
	r, err := name.ParseReference(ref, name.Insecure)
	require.NoError(t, err)
	desc, err := remote.Head(r)
	require.NoError(t, err)
	return r.Context().Digest(desc.Digest.String()), *desc
}

// signBundle attaches a Sigstore bundle to the artifact ref, as
// `cosign sign` >= v3 does.
func signBundle(t *testing.T, ref string, kp sign.Keypair, predicateType string) {
	t.Helper()
	digest, desc := subject(t, ref)
	statement, err := json.Marshal(map[string]any{
		"_type":         "https://in-toto.io/Statement/v1",
		"subject":       []any{map[string]any{"name": digest.Context().String(), "digest": map[string]string{"sha256": desc.Digest.Hex}}},
		"predicateType": predicateType,
		"predicate":     map[string]any{},
	})
	require.NoError(t, err)
	b, err := sign.Bundle(&sign.DSSEData{Data: statement, PayloadType: "application/vnd.in-toto+json"}, kp, sign.BundleOptions{})
	require.NoError(t, err)
	content, err := protojson.Marshal(b)
	require.NoError(t, err)

	img, err := mutate.AppendLayers(empty.Image, static.NewLayer(content, bundleMediaType))
	require.NoError(t, err)
	img = mutate.MediaType(img, types.OCIManifestSchema1)
	// The test registry uses the config media type as artifact type
	img = mutate.ConfigMediaType(img, bundleMediaType)
	img = mutate.Subject(img, desc).(v1.Image)
	imgDigest, err := img.Digest()
	require.NoError(t, err)
	require.NoError(t, remote.Write(digest.Context().Digest(imgDigest.String()), img))
}

// signLegacy pushes a signature in the sha256-<digest>.sig tag, as
// `cosign sign` < v3 does.
func signLegacy(t *testing.T, ref string, kp sign.Keypair, signedDigest string) {
	t.Helper()
	digest, _ := subject(t, ref)
	if signedDigest == "" {
		signedDigest = digest.DigestStr()
	}
	payload, err := json.Marshal(map[string]any{
		"critical": map[string]any{
			"identity": map[string]string{"docker-reference": digest.Context().String()},
			"image":    map[string]string{"docker-manifest-digest": signedDigest},
			"type":     "cosign container image signature",
		},
		"optional": nil,
	})
	require.NoError(t, err)
	sig, _, err := kp.SignData(context.Background(), payload)
	require.NoError(t, err)

	img, err := mutate.Append(empty.Image, mutate.Addendum{
		Layer:       static.NewLayer(payload, simpleSigningMediaType),
		Annotations: map[string]string{signatureAnnotation: base64.StdEncoding.EncodeToString(sig)},
	})
	require.NoError(t, err)
	tag := digest.Context().Tag(strings.Replace(digest.DigestStr(), ":", "-", 1) + ".sig")
	require.NoError(t, remote.Write(tag, img))
}

func newSignedRepository(t *testing.T, host string, publicKeyPaths ...string) *repository {
	t.Helper()
	r, err := New(cominTypes.OCIConfig{
		Path:                 t.TempDir(),
		Remotes:              []cominTypes.OCIRemote{newRemote("origin", host, cominTypes.OCIRef{Tag: "latest"}, cominTypes.OCIRef{})},
		CosignPublicKeyPaths: publicKeyPaths,
	}, MainArtifact{})
	require.NoError(t, err)
	return r
}

func TestCosignBundle(t *testing.T) {
	host := newRegistry(t)
	ref := host + "/owner/infra:latest"
	digest := push(t, ref, t1, map[string]string{"flake.nix": "1"})
	kp, keyPath := newKeypair(t)
	_, otherKeyPath := newKeypair(t)

	r := newSignedRepository(t, host, otherKeyPath, keyPath)

	// The artifact is not signed yet: it is not selected nor extracted
	s := fetch(t, r, "origin")
	assert.Equal(t, "", s.SelectedDigest)
	assert.Contains(t, s.ErrorMsg, "is not signed by a trusted cosign key")
	assert.NoDirExists(t, ArtifactDir(r.config.Path, digest))

	// An SBOM attestation is not a signature
	signBundle(t, ref, kp, "https://spdx.dev/Document")
	s = fetch(t, r, "origin")
	assert.Equal(t, "", s.SelectedDigest)
	assert.Contains(t, s.ErrorMsg, "predicate type")

	// Once signed, the artifact is selected on the next fetch
	signBundle(t, ref, kp, cosignSignPredicateType)
	s = fetch(t, r, "origin")
	assert.Equal(t, "", s.ErrorMsg)
	assert.Equal(t, digest, s.SelectedDigest)
	assert.True(t, s.SelectedShouldBeSigned.GetValue())
	assert.True(t, s.SelectedSigned.GetValue())
	assert.Equal(t, keyPath, s.SelectedSignedBy)
	assert.DirExists(t, ArtifactDir(r.config.Path, digest))
}

func TestCosignBundleWrongKey(t *testing.T) {
	host := newRegistry(t)
	ref := host + "/owner/infra:latest"
	push(t, ref, t1, map[string]string{"flake.nix": "1"})
	kp, _ := newKeypair(t)
	_, trustedKeyPath := newKeypair(t)
	signBundle(t, ref, kp, cosignSignPredicateType)

	r := newSignedRepository(t, host, trustedKeyPath)
	s := fetch(t, r, "origin")
	assert.Equal(t, "", s.SelectedDigest)
	assert.Contains(t, s.ErrorMsg, "is not signed by a trusted cosign key")
}

func TestCosignBundleOtherArtifact(t *testing.T) {
	host := newRegistry(t)
	kp, keyPath := newKeypair(t)
	// A bundle signing another artifact is attached to the artifact
	push(t, host+"/owner/infra:other", t1, map[string]string{"flake.nix": "other"})
	signBundle(t, host+"/owner/infra:other", kp, cosignSignPredicateType)
	otherDigest, _ := subject(t, host+"/owner/infra:other")
	digest := push(t, host+"/owner/infra:latest", t2, map[string]string{"flake.nix": "1"})

	index, err := remote.Referrers(otherDigest)
	require.NoError(t, err)
	m, err := index.IndexManifest()
	require.NoError(t, err)
	require.Len(t, m.Manifests, 1)
	bundleImg, err := remote.Image(otherDigest.Context().Digest(m.Manifests[0].Digest.String()))
	require.NoError(t, err)
	d, desc := subject(t, host+"/owner/infra:latest")
	bundleImg = mutate.Subject(bundleImg, desc).(v1.Image)
	bundleDigest, err := bundleImg.Digest()
	require.NoError(t, err)
	require.NoError(t, remote.Write(d.Context().Digest(bundleDigest.String()), bundleImg))

	r := newSignedRepository(t, host, keyPath)
	s := fetch(t, r, "origin")
	assert.Equal(t, "", s.SelectedDigest)
	assert.NoDirExists(t, ArtifactDir(r.config.Path, digest))
	assert.Contains(t, s.ErrorMsg, "is not signed by a trusted cosign key")
	assert.Contains(t, s.ErrorMsg, "does not match any digest in statement")
}

func TestCosignLegacy(t *testing.T) {
	host := newRegistry(t)
	ref := host + "/owner/infra:latest"
	digest := push(t, ref, t1, map[string]string{"flake.nix": "1"})
	kp, keyPath := newKeypair(t)
	signLegacy(t, ref, kp, "")

	r := newSignedRepository(t, host, keyPath)
	s := fetch(t, r, "origin")
	assert.Equal(t, "", s.ErrorMsg)
	assert.Equal(t, digest, s.SelectedDigest)
	assert.True(t, s.SelectedSigned.GetValue())
	assert.Equal(t, keyPath, s.SelectedSignedBy)
}

func TestCosignLegacyWrongDigest(t *testing.T) {
	host := newRegistry(t)
	ref := host + "/owner/infra:latest"
	push(t, ref, t1, map[string]string{"flake.nix": "1"})
	kp, keyPath := newKeypair(t)
	// The signature payload is about another artifact
	signLegacy(t, ref, kp, "sha256:0000000000000000000000000000000000000000000000000000000000000000")

	r := newSignedRepository(t, host, keyPath)
	s := fetch(t, r, "origin")
	assert.Equal(t, "", s.SelectedDigest)
	assert.Contains(t, s.ErrorMsg, "is not a signature of")
}

func TestCosignInvalidPublicKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cosign.pub")
	require.NoError(t, os.WriteFile(path, []byte("not a key"), 0600))
	_, err := New(cominTypes.OCIConfig{CosignPublicKeyPaths: []string{path}}, MainArtifact{})
	assert.ErrorContains(t, err, "failed to parse the cosign public key")
}
