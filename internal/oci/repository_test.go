package oci

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nlewo/comin/internal/types"
	pb "github.com/nlewo/comin/pkg/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	t1 = "2026-01-01T00:00:00Z"
	t2 = "2026-01-02T00:00:00Z"
	t3 = "2026-01-03T00:00:00Z"
)

func newRepository(t *testing.T, main MainArtifact, remotes ...types.OCIRemote) *repository {
	t.Helper()
	r, err := New(types.OCIConfig{Path: t.TempDir(), Remotes: remotes}, main)
	require.NoError(t, err)
	return r
}

func TestFetchTag(t *testing.T) {
	host := newRegistry(t)
	digest := push(t, host+"/owner/infra:latest", t1, map[string]string{"flake.nix": "{}", "hosts/machine.nix": "{}"})
	r := newRepository(t, MainArtifact{}, newRemote("origin", host, types.OCIRef{Tag: "latest"}, types.OCIRef{}))

	s := fetch(t, r, "origin")
	assert.Equal(t, "", s.ErrorMsg)
	assert.Equal(t, "", s.Remotes[0].FetchErrorMsg)
	assert.True(t, s.Remotes[0].Fetched.GetValue())
	assert.Equal(t, digest, s.SelectedDigest)
	assert.Equal(t, "latest", s.SelectedTag)
	assert.Equal(t, "origin", s.SelectedRemoteName)
	assert.False(t, s.SelectedIsTesting.GetValue())
	assert.Equal(t, "main@sha1:"+t1, s.SelectedRevision)
	assert.Equal(t, "https://github.com/owner/infra", s.SelectedSourceUrl)
	assert.Equal(t, digest, s.MainDigest)
	assert.Equal(t, t1, s.MainCreated)

	content, err := os.ReadFile(filepath.Join(ArtifactDir(r.config.Path, digest), "hosts/machine.nix"))
	require.NoError(t, err)
	assert.Equal(t, "{}", string(content))

	// Nothing changes on the next fetch
	s = fetch(t, r, "origin")
	assert.Equal(t, digest, s.SelectedDigest)

	// A new artifact is selected and the previous one is kept
	// since it could be under evaluation
	digest2 := push(t, host+"/owner/infra:latest", t2, map[string]string{"flake.nix": "{ }"})
	s = fetch(t, r, "origin")
	assert.Equal(t, digest2, s.SelectedDigest)
	assert.DirExists(t, ArtifactDir(r.config.Path, digest2))
	assert.DirExists(t, ArtifactDir(r.config.Path, digest))

	digest3 := push(t, host+"/owner/infra:latest", t3, map[string]string{"flake.nix": "{  }"})
	s = fetch(t, r, "origin")
	assert.Equal(t, digest3, s.SelectedDigest)
	assert.NoDirExists(t, ArtifactDir(r.config.Path, digest))
}

func TestFetchSemver(t *testing.T) {
	host := newRegistry(t)
	push(t, host+"/owner/infra:1.0.0", t1, map[string]string{"flake.nix": "1"})
	digest := push(t, host+"/owner/infra:1.2.0", t2, map[string]string{"flake.nix": "2"})
	push(t, host+"/owner/infra:2.0.0", t3, map[string]string{"flake.nix": "3"})
	push(t, host+"/owner/infra:latest", t3, map[string]string{"flake.nix": "3"})

	r := newRepository(t, MainArtifact{}, newRemote("origin", host, types.OCIRef{Semver: ">=1.0.0 <2.0.0"}, types.OCIRef{}))
	s := fetch(t, r, "origin")
	assert.Equal(t, digest, s.SelectedDigest)
	assert.Equal(t, "1.2.0", s.SelectedTag)
	assert.Equal(t, "1.2.0", s.MainTag)
	assert.Equal(t, "semver:>=1.0.0 <2.0.0", s.Remotes[0].Main.Spec)
}

func TestFetchSemverFilter(t *testing.T) {
	host := newRegistry(t)
	digest := push(t, host+"/owner/infra:1.0.0-machine", t1, map[string]string{"flake.nix": "1"})
	push(t, host+"/owner/infra:1.1.0-other", t2, map[string]string{"flake.nix": "2"})

	r := newRepository(t, MainArtifact{}, newRemote("origin", host, types.OCIRef{Semver: ">=1.0.0-0", SemverFilter: ".*-machine$"}, types.OCIRef{}))
	s := fetch(t, r, "origin")
	assert.Equal(t, digest, s.SelectedDigest)
	assert.Equal(t, "1.0.0-machine", s.SelectedTag)
}

func TestFetchDigest(t *testing.T) {
	host := newRegistry(t)
	digest := push(t, host+"/owner/infra:1.0.0", t2, map[string]string{"flake.nix": "1"})
	push(t, host+"/owner/infra:1.0.0", t3, map[string]string{"flake.nix": "2"})

	// An artifact pinned by digest is never refused, even if it is
	// older than the main artifact.
	r := newRepository(t, MainArtifact{Digest: "sha256:older", Created: t3}, newRemote("origin", host, types.OCIRef{Digest: digest}, types.OCIRef{}))
	s := fetch(t, r, "origin")
	assert.Equal(t, digest, s.SelectedDigest)
	assert.Equal(t, "", s.SelectedTag)
}

func TestFetchNotFound(t *testing.T) {
	host := newRegistry(t)
	push(t, host+"/owner/infra:latest", t1, map[string]string{"flake.nix": "1"})

	r := newRepository(t, MainArtifact{}, newRemote("origin", host, types.OCIRef{Tag: "missing"}, types.OCIRef{}))
	s := fetch(t, r, "origin")
	// A missing tag is not a fetch error
	assert.Equal(t, "", s.Remotes[0].FetchErrorMsg)
	assert.Contains(t, s.Remotes[0].Main.ErrorMsg, "not found")
	assert.Equal(t, "", s.SelectedDigest)

	r = newRepository(t, MainArtifact{}, newRemote("origin", host, types.OCIRef{Semver: ">=1.0.0"}, types.OCIRef{}))
	s = fetch(t, r, "origin")
	assert.Equal(t, "", s.Remotes[0].FetchErrorMsg)
	assert.Contains(t, s.Remotes[0].Main.ErrorMsg, "no tag matches")
	assert.Equal(t, "", s.SelectedDigest)
}

func TestFetchMirrors(t *testing.T) {
	host := newRegistry(t)
	digest := push(t, host+"/owner/infra:latest", t1, map[string]string{"flake.nix": "1"})

	// The first remote is down: the artifact is fetched from the second one
	r := newRepository(t, MainArtifact{},
		newRemote("down", "127.0.0.1:1", types.OCIRef{Tag: "latest"}, types.OCIRef{}),
		newRemote("mirror", host, types.OCIRef{Tag: "latest"}, types.OCIRef{}),
	)
	s := fetch(t, r, "down", "mirror")
	assert.NotEmpty(t, s.Remotes[0].FetchErrorMsg)
	assert.False(t, s.Remotes[0].Fetched.GetValue())
	assert.Equal(t, digest, s.SelectedDigest)
	assert.Equal(t, "mirror", s.SelectedRemoteName)
}

func TestFetchMainAndTesting(t *testing.T) {
	host := newRegistry(t)
	mainDigest := push(t, host+"/owner/infra:latest", t1, map[string]string{"flake.nix": "main"})
	r := newRepository(t, MainArtifact{}, newRemote("origin", host, types.OCIRef{Tag: "latest"}, types.OCIRef{Tag: "testing-machine"}))

	// The testing tag doesn't exist yet
	s := fetch(t, r, "origin")
	assert.Equal(t, mainDigest, s.SelectedDigest)
	assert.Contains(t, s.Remotes[0].Testing.ErrorMsg, "not found")

	// A testing artifact newer than the main one is preferred
	testingDigest := push(t, host+"/owner/infra:testing-machine", t2, map[string]string{"flake.nix": "testing"})
	s = fetch(t, r, "origin")
	assert.Equal(t, testingDigest, s.SelectedDigest)
	assert.True(t, s.SelectedIsTesting.GetValue())
	assert.Equal(t, "testing-machine", s.SelectedTag)
	assert.Equal(t, mainDigest, s.MainDigest)

	// Once a newer main artifact is pushed, the testing artifact
	// is older than main and is no longer selected
	mainDigest2 := push(t, host+"/owner/infra:latest", t3, map[string]string{"flake.nix": "main2"})
	s = fetch(t, r, "origin")
	assert.Equal(t, mainDigest2, s.SelectedDigest)
	assert.False(t, s.SelectedIsTesting.GetValue())
	assert.Equal(t, mainDigest2, s.MainDigest)
	assert.Contains(t, s.Remotes[0].Testing.ErrorMsg, "before the main artifact")
}

func TestFetchTestingEqualsMain(t *testing.T) {
	host := newRegistry(t)
	digest := push(t, host+"/owner/infra:latest", t1, map[string]string{"flake.nix": "main"})
	push(t, host+"/owner/infra:testing-machine", t1, map[string]string{"flake.nix": "main"})

	r := newRepository(t, MainArtifact{}, newRemote("origin", host, types.OCIRef{Tag: "latest"}, types.OCIRef{Tag: "testing-machine"}))
	s := fetch(t, r, "origin")
	assert.Equal(t, digest, s.SelectedDigest)
	assert.False(t, s.SelectedIsTesting.GetValue())
}

func TestFetchRefuseOlderArtifacts(t *testing.T) {
	host := newRegistry(t)
	push(t, host+"/owner/infra:latest", t1, map[string]string{"flake.nix": "1"})

	// The deployed main artifact is newer than the latest tag
	main := MainArtifact{Digest: "sha256:deployed", Created: t2}
	r := newRepository(t, main, newRemote("origin", host, types.OCIRef{Tag: "latest"}, types.OCIRef{}))
	s := fetch(t, r, "origin")
	assert.Equal(t, "", s.SelectedDigest)
	assert.Equal(t, "sha256:deployed", s.MainDigest)
	assert.Contains(t, s.Remotes[0].Main.ErrorMsg, "before the main artifact sha256:deployed")

	digest := push(t, host+"/owner/infra:latest", t3, map[string]string{"flake.nix": "3"})
	s = fetch(t, r, "origin")
	assert.Equal(t, digest, s.SelectedDigest)
	assert.Equal(t, "", s.Remotes[0].Main.ErrorMsg)
}

func TestFetchRefuseLowerVersions(t *testing.T) {
	host := newRegistry(t)
	push(t, host+"/owner/infra:1.2.0", t3, map[string]string{"flake.nix": "1"})

	main := MainArtifact{Digest: "sha256:deployed", Tag: "1.3.0", Created: t1}
	r := newRepository(t, main, newRemote("origin", host, types.OCIRef{Semver: ">=1.0.0"}, types.OCIRef{}))
	s := fetch(t, r, "origin")
	assert.Equal(t, "", s.SelectedDigest)
	assert.Contains(t, s.Remotes[0].Main.ErrorMsg, "the version 1.2.0 is lower than the version 1.3.0")
}

func TestFetchUnsupportedLayer(t *testing.T) {
	host := newRegistry(t)
	push(t, host+"/owner/infra:latest", t1, map[string]string{"flake.nix": "1"})

	remote := newRemote("origin", host, types.OCIRef{Tag: "latest"}, types.OCIRef{})
	remote.LayerMediaType = "application/vnd.unknown"
	r := newRepository(t, MainArtifact{}, remote)
	s := fetch(t, r, "origin")
	// The selection is not updated in order to retry on the next fetch
	assert.Equal(t, "", s.SelectedDigest)
	assert.Equal(t, "", s.MainDigest)
	assert.Contains(t, s.ErrorMsg, "no layer with the media type application/vnd.unknown")
}

func TestCheckNotOlder(t *testing.T) {
	main := MainArtifact{Digest: "sha256:main", Tag: "1.2.0", Created: t2}
	tests := []struct {
		name      string
		ref       types.OCIRef
		candidate *pb.OCIReference
		older     bool
	}{
		{"no main", types.OCIRef{Tag: "latest"}, &pb.OCIReference{Digest: "sha256:c", Created: t1}, false},
		{"same digest", types.OCIRef{Tag: "latest"}, &pb.OCIReference{Digest: "sha256:main", Created: t1}, false},
		{"newer", types.OCIRef{Tag: "latest"}, &pb.OCIReference{Digest: "sha256:c", Created: t3}, false},
		{"older", types.OCIRef{Tag: "latest"}, &pb.OCIReference{Digest: "sha256:c", Created: t1}, true},
		{"no created annotation", types.OCIRef{Tag: "latest"}, &pb.OCIReference{Digest: "sha256:c"}, false},
		{"pinned by digest", types.OCIRef{Digest: "sha256:c"}, &pb.OCIReference{Digest: "sha256:c", Created: t1}, false},
		{"higher version", types.OCIRef{Semver: "*"}, &pb.OCIReference{Digest: "sha256:c", Tag: "1.3.0", Created: t1}, false},
		{"lower version", types.OCIRef{Semver: "*"}, &pb.OCIReference{Digest: "sha256:c", Tag: "1.1.0", Created: t3}, true},
		{"same version and older", types.OCIRef{Semver: "*"}, &pb.OCIReference{Digest: "sha256:c", Tag: "1.2.0", Created: t1}, true},
		{"same version and newer", types.OCIRef{Semver: "*"}, &pb.OCIReference{Digest: "sha256:c", Tag: "1.2.0", Created: t3}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := main
			if tt.name == "no main" {
				m = MainArtifact{}
			}
			err := checkNotOlder(tt.ref, tt.candidate, m)
			if tt.older {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHighestSemverTag(t *testing.T) {
	tags := []string{"latest", "1.0.0", "v1.10.0", "1.9.0", "2.0.0-rc.1", "1.11.0-machine"}
	tag, err := highestSemverTag(tags, ">=1.0.0", "")
	require.NoError(t, err)
	assert.Equal(t, "v1.10.0", tag)

	tag, err = highestSemverTag(tags, ">=1.0.0-0", "-machine$")
	require.NoError(t, err)
	assert.Equal(t, "1.11.0-machine", tag)

	_, err = highestSemverTag(tags, ">=3.0.0", "")
	assert.ErrorIs(t, err, errNotFound)
}

func TestRefSpec(t *testing.T) {
	assert.Equal(t, "tag:latest", refSpec(types.OCIRef{Tag: "latest"}))
	assert.Equal(t, "semver:>=1.0.0", refSpec(types.OCIRef{Semver: ">=1.0.0"}))
	assert.Equal(t, "digest:sha256:abc", refSpec(types.OCIRef{Digest: "sha256:abc"}))
	assert.Equal(t, "", refSpec(types.OCIRef{}))
}
