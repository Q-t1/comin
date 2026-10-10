package types

const OperationTest = "test"
const OperationSwitch = "switch"
const OperationBoot = "boot"
const OperationNull = "null"

type Remote struct {
	Name     string
	URL      string
	Auth     Auth
	Branches Branches `yaml:"branches"`
	Timeout  int      `yaml:"timeout"`
	// The period to poll the remote in second
	Poller Poller `yaml:"poller"`
}

type Poller struct {
	Period int `yaml:"period"`
}

type GitConfig struct {
	// The repository Path
	Path string
	// The directory in the repository
	Dir                   string
	Remotes               []Remote
	GpgPublicKeyPaths     []string
	SshAllowedSignersPath string
	Submodules            bool
}

// OCIRemote is an OCI repository containing the configuration to deploy
// as an artifact.
type OCIRemote struct {
	Name string `yaml:"name"`
	// The URL of the repository, such as oci://ghcr.io/owner/infra
	URL  string  `yaml:"url"`
	Auth OCIAuth `yaml:"auth"`
	Ref  OCIRefs `yaml:"ref"`
	// The media type of the layer containing the configuration. When
	// empty, the first layer with a supported media type is used.
	LayerMediaType string `yaml:"layer_media_type"`
	// Insecure allows to use HTTP instead of HTTPS
	Insecure bool   `yaml:"insecure"`
	Timeout  int    `yaml:"timeout"`
	Poller   Poller `yaml:"poller"`
}

// OCIRef describes how an OCI artifact is selected. Only one of Tag,
// Semver and Digest can be set.
type OCIRef struct {
	Tag string `yaml:"tag"`
	// Semver is a semver constraint. The highest tag matching it is selected.
	Semver string `yaml:"semver"`
	// SemverFilter is a regular expression tags have to match to be
	// considered by the semver constraint.
	SemverFilter string `yaml:"semver_filter"`
	Digest       string `yaml:"digest"`
	Operation    string `yaml:"operation"`
}

// IsDefined returns true when the reference selects an artifact
func (r OCIRef) IsDefined() bool {
	return r.Tag != "" || r.Semver != "" || r.Digest != ""
}

type OCIRefs struct {
	Main    OCIRef `yaml:"main"`
	Testing OCIRef `yaml:"testing"`
}

type OCIAuth struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"-"`
	PasswordPath string `yaml:"password_path"`
	// The path of a Docker config.json file containing the registry credentials
	DockerConfigPath string `yaml:"docker_config_path"`
}

type OCIConfig struct {
	// The directory where artifacts are extracted
	Path string
	// The directory in the artifact
	Dir                  string
	Remotes              []OCIRemote
	CosignPublicKeyPaths []string
}

type Auth struct {
	AccessToken       string
	AccessTokenPath   string `yaml:"access_token_path"`
	SshDeployKeyPath  string `yaml:"ssh_deploy_key_path"`
	SshKnownHostsPath string `yaml:"ssh_known_hosts_path"`
	Username          string
}

type Branch struct {
	Name string `yaml:"name"`
	// TODO: use it
	Protected bool   `yaml:"protected"`
	Operation string `yaml:"operation"`
}

type Branches struct {
	Main    Branch `yaml:"main"`
	Testing Branch `yaml:"testing"`
}

type HttpServer struct {
	ListenAddress string `yaml:"listen_address"`
	Port          int    `yaml:"port"`
}

type Grpc struct {
	UnixSocketPath string `yaml:"unix_socket_path"`
}

type Confirmer struct {
	Mode         string `yaml:"mode"`
	AutoDuration int    `yaml:"autoconfirm_duration"`
}

type Retention struct {
	DeploymentBootEntryCapacity  int `yaml:"deployment_boot_entry_capacity"`
	DeploymentSuccessfulCapacity int `yaml:"deployment_successful_capacity"`
	DeploymentAnyCapacity        int `yaml:"deployment_any_capacity"`
}

type Configuration struct {
	Hostname      string `yaml:"hostname"`
	StateDir      string `yaml:"state_dir"`
	StateFilepath string `yaml:"state_filepath"`
	// RepositoryType describes type of the repository. It can currently only be "flake"
	RepositoryType        string      `yaml:"repository_type"`
	RepositorySubdir      string      `yaml:"repository_subdir"`
	Submodules            bool        `yaml:"submodules"`
	SystemAttr            string      `yaml:"system_attr"`
	Remotes               []Remote    `yaml:"remotes"`
	OCIRemotes            []OCIRemote `yaml:"oci_remotes"`
	ApiServer             HttpServer  `yaml:"api_server"`
	Grpc                  Grpc        `yaml:"grpc"`
	Exporter              HttpServer  `yaml:"exporter"`
	GpgPublicKeyPaths     []string    `yaml:"gpg_public_key_paths"`
	SshAllowedSignersPath string      `yaml:"ssh_allowed_signers_path"`
	CosignPublicKeyPaths  []string    `yaml:"cosign_public_key_paths"`
	PostDeploymentCommand string      `yaml:"post_deployment_command"`
	PostBuildCommand      string      `yaml:"post_build_command"`
	BuildConfirmer        Confirmer   `yaml:"build_confirmer"`
	DeployConfirmer       Confirmer   `yaml:"deploy_confirmer"`
	Retention             Retention   `yaml:"retention"`
	EvalTimeout           int         `yaml:"eval_timeout"`
	BuildTimeout          int         `yaml:"build_timeout"`
	BuildAttemptsLimit    int         `yaml:"build_attempts_limit"`
	BuildRetryTimer       int         `yaml:"build_retry_timer"`
}
