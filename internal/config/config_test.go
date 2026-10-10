package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nlewo/comin/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	configPath := "./configuration.yaml"
	expected := types.Configuration{
		Hostname:              "machine",
		StateDir:              "/var/lib/comin",
		StateFilepath:         "/var/lib/comin/state.json",
		PostDeploymentCommand: "/some/path",
		RepositoryType:        "flake",
		RepositorySubdir:      ".",
		Remotes: []types.Remote{
			{
				Name: "origin",
				URL:  "https://framagit.org/owner/infra",
				Auth: types.Auth{
					AccessToken:     "my-secret",
					AccessTokenPath: "./secret",
					Username:        "comin",
				},
				Timeout: 300,
				Branches: types.Branches{
					Main:    types.Branch{Operation: "switch"},
					Testing: types.Branch{Operation: "test"},
				},
			},
			{
				Name: "local",
				URL:  "/home/owner/git/infra",
				Auth: types.Auth{
					AccessToken:     "",
					AccessTokenPath: "",
					Username:        "comin",
				},
				Timeout: 300,
				Branches: types.Branches{
					Main:    types.Branch{Operation: "switch"},
					Testing: types.Branch{Operation: "test"},
				},
			},
		},
		ApiServer: types.HttpServer{
			ListenAddress: "127.0.0.1",
			Port:          4242,
		},
		Exporter: types.HttpServer{
			ListenAddress: "0.0.0.0",
			Port:          4243,
		},
		Grpc: types.Grpc{
			UnixSocketPath: "/var/lib/comin/grpc.sock",
		},
		EvalTimeout:    1800,
		BuildTimeout:   1800,
		BuildRetryTimer: 30,
	}
	config, err := Read(configPath)
	assert.Nil(t, err)
	assert.Equal(t, expected, config)
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "comin.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigOCIRemotes(t *testing.T) {
	passwordPath := filepath.Join(t.TempDir(), "password")
	assert.NoError(t, os.WriteFile(passwordPath, []byte("token\n"), 0600))
	path := writeConfig(t, `
hostname: machine
state_dir: /var/lib/comin
repository_type: flake
cosign_public_key_paths: [/etc/comin/cosign.pub]
oci_remotes:
  - name: ghcr
    url: oci://ghcr.io/owner/infra
    auth:
      password_path: `+passwordPath+`
    ref:
      testing:
        tag: testing-machine
  - name: mirror
    url: oci://registry.example.com/owner/infra
    ref:
      main:
        semver: ">=1.0.0"
        operation: boot
`)
	config, err := Read(path)
	assert.NoError(t, err)
	assert.Equal(t, []string{"/etc/comin/cosign.pub"}, config.CosignPublicKeyPaths)
	assert.Equal(t, []types.OCIRemote{
		{
			Name:    "ghcr",
			URL:     "oci://ghcr.io/owner/infra",
			Auth:    types.OCIAuth{Username: "comin", Password: "token", PasswordPath: passwordPath},
			Timeout: 300,
			Ref: types.OCIRefs{
				Main:    types.OCIRef{Tag: "latest", Operation: "switch"},
				Testing: types.OCIRef{Tag: "testing-machine", Operation: "test"},
			},
		},
		{
			Name:    "mirror",
			URL:     "oci://registry.example.com/owner/infra",
			Timeout: 300,
			Ref: types.OCIRefs{
				Main:    types.OCIRef{Semver: ">=1.0.0", Operation: "boot"},
				Testing: types.OCIRef{Operation: "test"},
			},
		},
	}, config.OCIRemotes)

	ociConfig := MkOCIConfig(config)
	assert.Equal(t, "/var/lib/comin/oci", ociConfig.Path)
	assert.Equal(t, ".", ociConfig.Dir)
}

func TestConfigOCIRemotesErrors(t *testing.T) {
	tests := []struct {
		name   string
		config string
		err    string
	}{
		{
			"remotes and oci_remotes",
			"remotes: [{name: origin, url: https://example.com/infra}]\noci_remotes: [{name: ghcr, url: oci://ghcr.io/owner/infra}]",
			"remotes and oci_remotes cannot be both set",
		},
		{
			"missing scheme",
			"oci_remotes: [{name: ghcr, url: ghcr.io/owner/infra}]",
			"must start with oci://",
		},
		{
			"tag and semver",
			"oci_remotes: [{name: ghcr, url: oci://ghcr.io/owner/infra, ref: {main: {tag: latest, semver: '>=1.0.0'}}}]",
			"only one of tag, semver and digest can be set",
		},
		{
			"invalid semver",
			"oci_remotes: [{name: ghcr, url: oci://ghcr.io/owner/infra, ref: {testing: {semver: 'not a constraint'}}}]",
			"the testing reference of the OCI remote 'ghcr' is invalid",
		},
		{
			"filter without semver",
			"oci_remotes: [{name: ghcr, url: oci://ghcr.io/owner/infra, ref: {main: {tag: latest, semver_filter: '.*'}}}]",
			"semver_filter requires semver to be set",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Read(writeConfig(t, "hostname: machine\nrepository_type: flake\n"+tt.config))
			assert.ErrorContains(t, err, tt.err)
		})
	}
}
