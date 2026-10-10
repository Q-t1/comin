package oci

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sirupsen/logrus"
)

const (
	// The prefix of the media type of Sigstore bundles, which are
	// attached to artifacts as OCI referrers by cosign >= v3.
	bundleMediaTypePrefix = "application/vnd.dev.sigstore.bundle"
	// The predicate type of the in-toto statement signed by `cosign sign`
	cosignSignPredicateType = "https://sigstore.dev/cosign/sign/v1"

	// The legacy signature format of cosign < v3: signatures are
	// stored in the sha256-<digest>.sig tag.
	simpleSigningMediaType = "application/vnd.dev.cosign.simplesigning.v1+json"
	signatureAnnotation    = "dev.cosignproject.cosign/signature"
	simpleSigningType      = "cosign container image signature"
)

// cosignPublicKey is a public key used to verify cosign signatures
type cosignPublicKey struct {
	path     string
	verifier signature.Verifier
}

// cosignVerifier verifies artifacts are signed with cosign by one of
// its public keys. Both the Sigstore bundle format (cosign >= v3) and
// the legacy format are supported. Since public keys are the trust
// anchor, transparency log entries are not verified (as with `cosign
// verify --key --insecure-ignore-tlog`): verifications don't require
// any network access to the Sigstore infrastructure.
type cosignVerifier struct {
	keys []cosignPublicKey
}

func newCosignVerifier(publicKeyPaths []string) (*cosignVerifier, error) {
	v := &cosignVerifier{}
	for _, path := range publicKeyPaths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read the cosign public key %s: %w", path, err)
		}
		publicKey, err := cryptoutils.UnmarshalPEMToPublicKey(content)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the cosign public key %s: %w", path, err)
		}
		verifier, err := signature.LoadDefaultVerifier(publicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load the cosign public key %s: %w", path, err)
		}
		v.keys = append(v.keys, cosignPublicKey{path: path, verifier: verifier})
	}
	return v, nil
}

// verify returns the path of the public key which verifies a signature
// of the artifact digest. An error is returned if no signature has
// been verified.
func (v *cosignVerifier) verify(repo name.Repository, digest string, opts []remote.Option) (signedBy string, err error) {
	signedBy, bundleErr := v.verifyBundles(repo, digest, opts)
	if bundleErr == nil {
		return signedBy, nil
	}
	signedBy, legacyErr := v.verifyLegacySignatures(repo, digest, opts)
	if legacyErr == nil {
		return signedBy, nil
	}
	return "", errors.Join(bundleErr, legacyErr)
}

// verifyBundles verifies the Sigstore bundles attached to the artifact
// as OCI referrers.
func (v *cosignVerifier) verifyBundles(repo name.Repository, digest string, opts []remote.Option) (string, error) {
	artifactDigest, err := hex.DecodeString(strings.TrimPrefix(digest, "sha256:"))
	if err != nil {
		return "", err
	}
	index, err := remote.Referrers(repo.Digest(digest), opts...)
	if err != nil {
		return "", fmt.Errorf("failed to get the referrers of %s: %w", digest, err)
	}
	manifest, err := index.IndexManifest()
	if err != nil {
		return "", err
	}
	errs := []error{fmt.Errorf("no valid cosign bundle has been found")}
	for _, desc := range manifest.Manifests {
		if !strings.HasPrefix(desc.ArtifactType, bundleMediaTypePrefix) {
			continue
		}
		b, err := fetchBundle(repo, desc, opts)
		if err != nil {
			errs = append(errs, fmt.Errorf("bundle %s: %w", desc.Digest, err))
			continue
		}
		for _, key := range v.keys {
			if err := verifyBundle(b, key.verifier, artifactDigest); err != nil {
				errs = append(errs, fmt.Errorf("bundle %s with key %s: %w", desc.Digest, key.path, err))
				continue
			}
			logrus.Infof("oci: the artifact %s is signed by %s", digest, key.path)
			return key.path, nil
		}
	}
	return "", errors.Join(errs...)
}

func fetchBundle(repo name.Repository, desc v1.Descriptor, opts []remote.Option) (*bundle.Bundle, error) {
	img, err := remote.Image(repo.Digest(desc.Digest.String()), opts...)
	if err != nil {
		return nil, err
	}
	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}
	if len(layers) != 1 {
		return nil, fmt.Errorf("the bundle manifest has %d layers while 1 is expected", len(layers))
	}
	content, err := readLayer(layers[0])
	if err != nil {
		return nil, err
	}
	var b bundle.Bundle
	if err := b.UnmarshalJSON(content); err != nil {
		return nil, err
	}
	return &b, nil
}

func verifyBundle(b *bundle.Bundle, key signature.Verifier, artifactDigest []byte) error {
	trustedMaterial := root.NewTrustedPublicKeyMaterial(func(string) (root.TimeConstrainedVerifier, error) {
		// Keys have no validity period
		return root.NewExpiringKey(key, time.Time{}, time.Time{}), nil
	})
	verifier, err := verify.NewVerifier(trustedMaterial, verify.WithNoObserverTimestamps())
	if err != nil {
		return err
	}
	result, err := verifier.Verify(b, verify.NewPolicy(verify.WithArtifactDigest("sha256", artifactDigest), verify.WithKey()))
	if err != nil {
		return err
	}
	// Attestations (SBOM, provenance...) are also signed in-toto
	// statements about the artifact: only signatures are accepted.
	if result.Statement != nil && result.Statement.PredicateType != cosignSignPredicateType {
		return fmt.Errorf("the bundle predicate type is %s while %s is expected", result.Statement.PredicateType, cosignSignPredicateType)
	}
	return nil
}

// simpleSigning is the payload signed by cosign in the legacy format
type simpleSigning struct {
	Critical struct {
		Image struct {
			DockerManifestDigest string `json:"docker-manifest-digest"`
		} `json:"image"`
		Type string `json:"type"`
	} `json:"critical"`
}

// verifyLegacySignatures verifies the signatures stored in the
// sha256-<digest>.sig tag.
func (v *cosignVerifier) verifyLegacySignatures(repo name.Repository, digest string, opts []remote.Option) (string, error) {
	tag := strings.Replace(digest, ":", "-", 1) + ".sig"
	img, err := remote.Image(repo.Tag(tag), opts...)
	if err != nil {
		if isNotFound(err) {
			return "", fmt.Errorf("no legacy cosign signature has been found in the tag %s", tag)
		}
		return "", err
	}
	manifest, err := img.Manifest()
	if err != nil {
		return "", err
	}
	errs := []error{fmt.Errorf("no valid legacy cosign signature has been found in the tag %s", tag)}
	for _, desc := range manifest.Layers {
		if desc.MediaType != simpleSigningMediaType {
			continue
		}
		sig, err := base64.StdEncoding.DecodeString(desc.Annotations[signatureAnnotation])
		if err != nil {
			errs = append(errs, fmt.Errorf("signature %s: %w", desc.Digest, err))
			continue
		}
		layer, err := img.LayerByDigest(desc.Digest)
		if err != nil {
			return "", err
		}
		payload, err := readLayer(layer)
		if err != nil {
			return "", err
		}
		var s simpleSigning
		if err := json.Unmarshal(payload, &s); err != nil {
			errs = append(errs, fmt.Errorf("signature %s: %w", desc.Digest, err))
			continue
		}
		if s.Critical.Type != simpleSigningType || s.Critical.Image.DockerManifestDigest != digest {
			errs = append(errs, fmt.Errorf("signature %s is not a signature of %s", desc.Digest, digest))
			continue
		}
		for _, key := range v.keys {
			if err := key.verifier.VerifySignature(bytes.NewReader(sig), bytes.NewReader(payload)); err != nil {
				errs = append(errs, fmt.Errorf("signature %s with key %s: %w", desc.Digest, key.path, err))
				continue
			}
			logrus.Infof("oci: the artifact %s is signed by %s", digest, key.path)
			return key.path, nil
		}
	}
	return "", errors.Join(errs...)
}

// readLayer returns the content of a small layer, such as a signature
// payload or a bundle.
func readLayer(layer v1.Layer) ([]byte, error) {
	// The content is not compressed: Compressed returns it as is and
	// verifies its digest.
	rc, err := layer.Compressed()
	if err != nil {
		return nil, err
	}
	defer rc.Close() // nolint: errcheck
	return io.ReadAll(io.LimitReader(rc, 4<<20))
}
