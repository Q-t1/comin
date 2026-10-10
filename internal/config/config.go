package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/nlewo/comin/internal/types"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

func Read(path string) (config types.Configuration, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close() // nolint

	d := yaml.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return config, err
	}
	for i, remote := range config.Remotes {
		if remote.Auth.AccessTokenPath != "" {
			content, err := os.ReadFile(remote.Auth.AccessTokenPath)
			if err != nil {
				return config, err
			}
			config.Remotes[i].Auth.AccessToken = strings.TrimSpace(string(content))
		}
		// On GitLab and GitHub, any non blank username is working
		if remote.Auth.Username == "" {
			config.Remotes[i].Auth.Username = "comin"
		}
		if remote.Timeout == 0 {
			config.Remotes[i].Timeout = 300
		}
		if remote.Branches.Main.Operation == "" {
			config.Remotes[i].Branches.Main.Operation = "switch"
		}
		if remote.Branches.Testing.Operation == "" {
			config.Remotes[i].Branches.Testing.Operation = "test"
		}

	}
	if len(config.Remotes) > 0 && len(config.OCIRemotes) > 0 {
		return config, fmt.Errorf("config: remotes and oci_remotes cannot be both set")
	}
	for i := range config.OCIRemotes {
		if err := setOCIRemoteDefaults(&config.OCIRemotes[i]); err != nil {
			return config, err
		}
	}

	if config.ApiServer.ListenAddress == "" {
		config.ApiServer.ListenAddress = "127.0.0.1"
	}
	if config.ApiServer.Port == 0 {
		config.ApiServer.Port = 4242
	}
	if config.Exporter.ListenAddress == "" {
		config.Exporter.ListenAddress = "0.0.0.0"
	}
	if config.Exporter.Port == 0 {
		config.Exporter.Port = 4243
	}
	if config.StateFilepath == "" {
		config.StateFilepath = filepath.Join(config.StateDir, "state.json")
	}
	if config.RepositorySubdir == "" {
		config.RepositorySubdir = "."
	}
	supportedRepositoryTypes := []string{"flake", "nix"}
	if !slices.Contains(supportedRepositoryTypes, config.RepositoryType) {
		return config, fmt.Errorf("config: repository type is '%s' while it be one of '%s'", config.RepositoryType, supportedRepositoryTypes)
	}
	if config.Grpc.UnixSocketPath == "" {
		config.Grpc.UnixSocketPath = filepath.Join(config.StateDir, "grpc.sock")
	}
	if config.EvalTimeout == 0 {
		config.EvalTimeout = 1800
	}
	if config.BuildTimeout == 0 {
		config.BuildTimeout = 1800
	}
	if config.BuildRetryTimer == 0 {
		config.BuildRetryTimer = 30
	}
	logrus.Debugf("Config is '%#v'", config)
	return
}

func setOCIRemoteDefaults(remote *types.OCIRemote) error {
	if !strings.HasPrefix(remote.URL, "oci://") {
		return fmt.Errorf("config: the URL of the OCI remote '%s' must start with oci://", remote.Name)
	}
	if remote.Auth.PasswordPath != "" {
		content, err := os.ReadFile(remote.Auth.PasswordPath)
		if err != nil {
			return err
		}
		remote.Auth.Password = strings.TrimSpace(string(content))
		// On GitHub and GitLab registries, any non blank username is working
		if remote.Auth.Username == "" {
			remote.Auth.Username = "comin"
		}
	}
	if remote.Timeout == 0 {
		remote.Timeout = 300
	}
	if !remote.Ref.Main.IsDefined() {
		remote.Ref.Main.Tag = "latest"
	}
	if remote.Ref.Main.Operation == "" {
		remote.Ref.Main.Operation = "switch"
	}
	if remote.Ref.Testing.Operation == "" {
		remote.Ref.Testing.Operation = "test"
	}
	if err := validateOCIRef(remote.Ref.Main); err != nil {
		return fmt.Errorf("config: the main reference of the OCI remote '%s' is invalid: %w", remote.Name, err)
	}
	if err := validateOCIRef(remote.Ref.Testing); err != nil {
		return fmt.Errorf("config: the testing reference of the OCI remote '%s' is invalid: %w", remote.Name, err)
	}
	return nil
}

func validateOCIRef(ref types.OCIRef) error {
	set := 0
	for _, s := range []string{ref.Tag, ref.Semver, ref.Digest} {
		if s != "" {
			set++
		}
	}
	if set > 1 {
		return fmt.Errorf("only one of tag, semver and digest can be set")
	}
	if ref.Semver != "" {
		if _, err := semver.NewConstraint(ref.Semver); err != nil {
			return err
		}
	}
	if ref.SemverFilter != "" {
		if ref.Semver == "" {
			return fmt.Errorf("semver_filter requires semver to be set")
		}
		if _, err := regexp.Compile(ref.SemverFilter); err != nil {
			return err
		}
	}
	if ref.Digest != "" && !strings.HasPrefix(ref.Digest, "sha256:") {
		return fmt.Errorf("the digest %s is not a sha256 digest", ref.Digest)
	}
	return nil
}

func MkGitConfig(config types.Configuration) types.GitConfig {
	return types.GitConfig{
		Path:                  filepath.Join(config.StateDir, "repository"),
		Dir:                   config.RepositorySubdir,
		Remotes:               config.Remotes,
		GpgPublicKeyPaths:     config.GpgPublicKeyPaths,
		SshAllowedSignersPath: config.SshAllowedSignersPath,
		Submodules:            config.Submodules,
	}
}

func MkOCIConfig(config types.Configuration) types.OCIConfig {
	return types.OCIConfig{
		Path:                 filepath.Join(config.StateDir, "oci"),
		Dir:                  config.RepositorySubdir,
		Remotes:              config.OCIRemotes,
		CosignPublicKeyPaths: config.CosignPublicKeyPaths,
	}
}
