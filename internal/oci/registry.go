package oci

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/docker/cli/cli/config"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/nlewo/comin/internal/types"
)

// Annotations set by `flux push artifact`, `oras push` and most OCI
// artifact producers
const (
	AnnotationRevision = "org.opencontainers.image.revision"
	AnnotationSource   = "org.opencontainers.image.source"
	AnnotationCreated  = "org.opencontainers.image.created"
)

// artifact is an artifact resolved from a reference
type artifact struct {
	tag       string
	digest    string
	revision  string
	sourceURL string
	created   string
}

// errNotFound is returned when a reference doesn't match any artifact
var errNotFound = errors.New("not found")

// repositoryName parses the remote URL. The oci:// scheme is removed.
func repositoryName(remote types.OCIRemote) (name.Repository, error) {
	var opts []name.Option
	if remote.Insecure {
		opts = append(opts, name.Insecure)
	}
	return name.NewRepository(strings.TrimPrefix(remote.URL, "oci://"), opts...)
}

// authenticator returns the credentials used to access the registry
// of the repository.
func authenticator(auth types.OCIAuth, repo name.Repository) (authn.Authenticator, error) {
	if auth.Password != "" {
		return authn.FromConfig(authn.AuthConfig{
			Username: auth.Username,
			Password: auth.Password,
		}), nil
	}
	if auth.DockerConfigPath != "" {
		f, err := os.Open(auth.DockerConfigPath)
		if err != nil {
			return nil, err
		}
		defer f.Close() // nolint: errcheck
		cf, err := config.LoadFromReader(f)
		if err != nil {
			return nil, fmt.Errorf("failed to load the Docker config file %s: %w", auth.DockerConfigPath, err)
		}
		key := repo.RegistryStr()
		// The Docker Hub key is hardcoded in config files
		if key == name.DefaultRegistry {
			key = authn.DefaultAuthKey
		}
		cfg, err := cf.GetAuthConfig(key)
		if err != nil {
			return nil, err
		}
		return authn.FromConfig(authn.AuthConfig{
			Username:      cfg.Username,
			Password:      cfg.Password,
			Auth:          cfg.Auth,
			IdentityToken: cfg.IdentityToken,
			RegistryToken: cfg.RegistryToken,
		}), nil
	}
	return authn.Anonymous, nil
}

// remoteOptions returns the options used for all requests to a remote
func remoteOptions(ctx context.Context, remoteConfig types.OCIRemote, repo name.Repository) ([]remote.Option, error) {
	auth, err := authenticator(remoteConfig.Auth, repo)
	if err != nil {
		return nil, err
	}
	return []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuth(auth),
		remote.WithUserAgent("comin"),
	}, nil
}

// resolve resolves a reference to an artifact. The manifest is only
// downloaded if its digest differs from the digest of the previous
// artifact, which is returned otherwise.
func resolve(repo name.Repository, ref types.OCIRef, previous artifact, opts []remote.Option) (a artifact, err error) {
	var reference name.Reference
	switch {
	case ref.Digest != "":
		reference = repo.Digest(ref.Digest)
	case ref.Semver != "":
		tags, err := remote.List(repo, opts...)
		if err != nil {
			return a, err
		}
		a.tag, err = highestSemverTag(tags, ref.Semver, ref.SemverFilter)
		if err != nil {
			return a, err
		}
		reference = repo.Tag(a.tag)
	default:
		a.tag = ref.Tag
		reference = repo.Tag(ref.Tag)
	}

	// Registries such as the Docker Hub only count GET requests in
	// their rate limits, so the manifest is first HEADed.
	if desc, err := remote.Head(reference, opts...); err == nil {
		a.digest = desc.Digest.String()
		if a.digest == previous.digest {
			previous.tag = a.tag
			return previous, nil
		}
	}

	desc, err := remote.Get(reference, opts...)
	if err != nil {
		if isNotFound(err) {
			return a, fmt.Errorf("%s: %w", reference, errNotFound)
		}
		return a, err
	}
	if desc.MediaType.IsIndex() {
		return a, fmt.Errorf("%s is an image index while an image manifest is expected", reference)
	}
	a.digest = desc.Digest.String()
	manifest, err := v1.ParseManifest(bytes.NewReader(desc.Manifest))
	if err != nil {
		return a, err
	}
	a.revision = manifest.Annotations[AnnotationRevision]
	a.sourceURL = manifest.Annotations[AnnotationSource]
	a.created = manifest.Annotations[AnnotationCreated]
	return a, nil
}

func isNotFound(err error) bool {
	var terr *transport.Error
	if errors.As(err, &terr) {
		return terr.StatusCode == http.StatusNotFound
	}
	return false
}

// highestSemverTag returns the highest tag matching the semver
// constraint. If filter is not empty, only tags matching this
// regular expression are considered.
func highestSemverTag(tags []string, constraint, filter string) (string, error) {
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return "", err
	}
	var re *regexp.Regexp
	if filter != "" {
		if re, err = regexp.Compile(filter); err != nil {
			return "", err
		}
	}
	var highest *semver.Version
	var highestTag string
	for _, tag := range tags {
		if re != nil && !re.MatchString(tag) {
			continue
		}
		v, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		if !c.Check(v) {
			continue
		}
		if highest == nil || v.GreaterThan(highest) {
			highest = v
			highestTag = tag
		}
	}
	if highest == nil {
		return "", fmt.Errorf("no tag matches the semver constraint '%s': %w", constraint, errNotFound)
	}
	return highestTag, nil
}
