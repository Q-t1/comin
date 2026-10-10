package oci

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/nlewo/comin/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type entry struct {
	hdr     tar.Header
	content string
}

func tarball(t *testing.T, entries ...entry) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		hdr := e.hdr
		hdr.Size = int64(len(e.content))
		if hdr.Mode == 0 {
			hdr.Mode = 0644
		}
		require.NoError(t, tw.WriteHeader(&hdr))
		_, err := tw.Write([]byte(e.content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	return &buf
}

func TestExtract(t *testing.T) {
	dir := t.TempDir()
	err := extract(dir, tarball(t,
		entry{hdr: tar.Header{Name: "./", Typeflag: tar.TypeDir}},
		entry{hdr: tar.Header{Name: "./flake.nix", Typeflag: tar.TypeReg}, content: "flake"},
		// The parent directory is not in the archive
		entry{hdr: tar.Header{Name: "hosts/machine/default.nix", Typeflag: tar.TypeReg}, content: "machine"},
		entry{hdr: tar.Header{Name: "script.sh", Typeflag: tar.TypeReg, Mode: 0700}, content: "#!/bin/sh"},
		entry{hdr: tar.Header{Name: "link.nix", Typeflag: tar.TypeSymlink, Linkname: "flake.nix"}},
		entry{hdr: tar.Header{Name: "hard.nix", Typeflag: tar.TypeLink, Linkname: "flake.nix"}},
	))
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "hosts/machine/default.nix"))
	require.NoError(t, err)
	assert.Equal(t, "machine", string(content))
	content, err = os.ReadFile(filepath.Join(dir, "link.nix"))
	require.NoError(t, err)
	assert.Equal(t, "flake", string(content))
	content, err = os.ReadFile(filepath.Join(dir, "hard.nix"))
	require.NoError(t, err)
	assert.Equal(t, "flake", string(content))

	fi, err := os.Stat(filepath.Join(dir, "script.sh"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), fi.Mode().Perm())
	fi, err = os.Stat(filepath.Join(dir, "flake.nix"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), fi.Mode().Perm())
}

func TestExtractRefusesEscapes(t *testing.T) {
	outside := t.TempDir()
	tests := []struct {
		name    string
		entries []entry
	}{
		{"parent directory", []entry{
			{hdr: tar.Header{Name: "../evil", Typeflag: tar.TypeReg}, content: "evil"},
		}},
		{"absolute path", []entry{
			{hdr: tar.Header{Name: filepath.Join(outside, "evil"), Typeflag: tar.TypeReg}, content: "evil"},
		}},
		{"write through a symlink", []entry{
			{hdr: tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: outside}},
			{hdr: tar.Header{Name: "link/evil", Typeflag: tar.TypeReg}, content: "evil"},
		}},
		{"hard link to an outside file", []entry{
			{hdr: tar.Header{Name: "hard", Typeflag: tar.TypeLink, Linkname: "../../etc/passwd"}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := extract(t.TempDir(), tarball(t, tt.entries...))
			assert.Error(t, err)
			assert.NoFileExists(t, filepath.Join(outside, "evil"))
		})
	}
}

func TestGC(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"sha256-a", "sha256-b", "sha256-c", ".tmp-123"} {
		require.NoError(t, os.Mkdir(filepath.Join(dir, d), 0755))
	}
	gc(dir, []string{"sha256:a", "sha256:c", ""})
	assert.DirExists(t, filepath.Join(dir, "sha256-a"))
	assert.NoDirExists(t, filepath.Join(dir, "sha256-b"))
	assert.DirExists(t, filepath.Join(dir, "sha256-c"))
	assert.NoDirExists(t, filepath.Join(dir, ".tmp-123"))
}

func TestAuthenticatorDockerConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	auth := base64.StdEncoding.EncodeToString([]byte("user:secret"))
	require.NoError(t, os.WriteFile(configPath, []byte(`{"auths": {"ghcr.io": {"auth": "`+auth+`"}}}`), 0600))

	repo, err := name.NewRepository("ghcr.io/owner/infra")
	require.NoError(t, err)
	a, err := authenticator(types.OCIAuth{DockerConfigPath: configPath}, repo)
	require.NoError(t, err)
	cfg, err := a.Authorization()
	require.NoError(t, err)
	assert.Equal(t, "user", cfg.Username)
	assert.Equal(t, "secret", cfg.Password)

	// A registry not in the config file is accessed anonymously
	repo, err = name.NewRepository("quay.io/owner/infra")
	require.NoError(t, err)
	a, err = authenticator(types.OCIAuth{DockerConfigPath: configPath}, repo)
	require.NoError(t, err)
	cfg, err = a.Authorization()
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Username)
}

func TestAuthenticatorPassword(t *testing.T) {
	repo, err := name.NewRepository("ghcr.io/owner/infra")
	require.NoError(t, err)
	a, err := authenticator(types.OCIAuth{Username: "comin", Password: "token"}, repo)
	require.NoError(t, err)
	cfg, err := a.Authorization()
	require.NoError(t, err)
	assert.Equal(t, "comin", cfg.Username)
	assert.Equal(t, "token", cfg.Password)
}
