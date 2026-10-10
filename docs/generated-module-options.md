## assertions

*Type:*
anything



## services\.comin\.enable



Whether to run the comin service\.



*Type:*
boolean



*Default:*

```nix
false
```



## services\.comin\.package



The comin package to use\.



*Type:*
null or package



*Default:*

```nix
"pkgs.comin or comin.packages.\${system}.default or null"
```



## services\.comin\.buildAttemptsLimit



Maximum number of build attempts before giving up\. When set to 0,
comin will retry forever on build failures\.



*Type:*
signed integer



*Default:*

```nix
0
```



## services\.comin\.buildConfirmer



The confirmer options for the build\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.buildConfirmer\.autoconfirm_duration



The autoconfirm timer duration in seconds\. After this
duration, the action is automatically confirmed\.



*Type:*
signed integer



*Default:*

```nix
120
```



## services\.comin\.buildConfirmer\.mode



The confirmer mode\. “without” immediately confirms
without any user interaction\. “manual” requires a user
confirmation\. “auto” automatically confirms after
waiting for the autoconfirm_duration\.



*Type:*
one of “without”, “auto”, “manual”



*Default:*

```nix
"without"
```



## services\.comin\.buildRetryTimer



Duration in seconds between each build retry attempt\.



*Type:*
signed integer



*Default:*

```nix
30
```



## services\.comin\.buildTimeout



Maximum duration in seconds for a Nix build before comin cancels it\.



*Type:*
signed integer



*Default:*

```nix
1800
```



## services\.comin\.cosignPublicKeyPaths



A list of cosign public key file paths\. When set, an OCI
artifact is only deployed if it is signed by one of these
keys\. Both the Sigstore bundle format (cosign >= v3) and the
legacy format are supported\.



*Type:*
list of string



*Default:*

```nix
[ ]
```



## services\.comin\.debug



Whether to run comin in debug mode\. Be careful, secrets are shown!\.



*Type:*
boolean



*Default:*

```nix
false
```



## services\.comin\.deployConfirmer



The confirmer options for the deployment\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.deployConfirmer\.autoconfirm_duration



The autoconfirm timer duration in seconds\. After this
duration, the action is automatically confirmed\.



*Type:*
signed integer



*Default:*

```nix
120
```



## services\.comin\.deployConfirmer\.mode



The confirmer mode\. “without” immediately confirms
without any user interaction\. “manual” requires a user
confirmation\. “auto” automatically confirms after
waiting for the autoconfirm_duration\.



*Type:*
one of “without”, “auto”, “manual”



*Default:*

```nix
"without"
```



## services\.comin\.desktop\.enable



Whether to enable Whether to run the comin desktop service\. This user service send notifications over DBus…



*Type:*
boolean



*Default:*

```nix
false
```



*Example:*

```nix
true
```



## services\.comin\.desktop\.title



The notification title\.



*Type:*
string



*Default:*

```nix
"comin"
```



## services\.comin\.evalTimeout



Maximum duration in seconds for a Nix evaluation (flake/nix eval)
before comin cancels it\.



*Type:*
signed integer



*Default:*

```nix
1800
```



## services\.comin\.exporter



Options for the Prometheus exporter\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.exporter\.listen_address



Address to listen on for the Prometheus exporter\. Empty string will listen on all interfaces\.



*Type:*
string



*Default:*

```nix
""
```



## services\.comin\.exporter\.openFirewall



Open port in firewall for incoming connections to the Prometheus exporter\.



*Type:*
boolean



*Default:*

```nix
false
```



## services\.comin\.exporter\.port



Port to listen on for the Prometheus exporter\.



*Type:*
signed integer



*Default:*

```nix
4243
```



## services\.comin\.gpgPublicKeyPaths



A list of GPG public key file paths\. Each of this file should contains an armored GPG key\.



*Type:*
list of string



*Default:*

```nix
[ ]
```



## services\.comin\.hostname



The name of the configuration to evaluate and deploy\.
This value is used by comin to evaluate the flake output
nixosConfigurations\.“\<hostname>” or darwinConfigurations\.“\<hostname>”\.
Defaults to networking\.hostName - you MUST set either this option
or networking\.hostName in your configuration\.



*Type:*
string



*Default:*

```nix
config.networking.hostName
```



## services\.comin\.machineId



The expected machine-id of the machine configured by
comin\. If not null, the configuration is only deployed
when this specified machine-id is equal to the actual
machine-id\.
This is mainly useful for server migration: this allows
to migrate a configuration from a machine to another
machine (with different hardware for instance) without
impacting both\.
Note it is only used by comin at evaluation\.



*Type:*
null or string



*Default:*

```nix
null
```



## services\.comin\.ociRemotes



Ordered list of OCI repositories to pull the configuration
from, as an alternative to Git ` remotes `\. The configuration
is an OCI artifact, such as pushed by ` flux push artifact `
or ` oras push `\. Several repositories can be used to pull
from registry mirrors\.



*Type:*
list of (submodule)



*Default:*

```nix
[ ]
```



*Example:*

```nix
[{
  name = "ghcr";
  url = "oci://ghcr.io/owner/infra";
  ref.main.semver = ">=1.0.0";
  auth.password_path = "/run/secrets/ghcr-token";
}]

```



## services\.comin\.ociRemotes\.\*\.auth



Authentication options\. The registry is accessed anonymously by default\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.ociRemotes\.\*\.auth\.docker_config_path



The path of a Docker ` config.json ` file
containing the registry credentials, which can
be used instead of ` password_path `\.



*Type:*
null or string



*Default:*

```nix
null
```



## services\.comin\.ociRemotes\.\*\.auth\.password_path



The path of a file containing the registry password or token\.



*Type:*
null or string



*Default:*

```nix
null
```



## services\.comin\.ociRemotes\.\*\.auth\.username



The username used with the password\. Note that any
non empty username is valid on GitHub and GitLab
registries\.



*Type:*
string



*Default:*

```nix
"comin"
```



## services\.comin\.ociRemotes\.\*\.insecure



Whether to use HTTP instead of HTTPS to access the registry\.



*Type:*
boolean



*Default:*

```nix
false
```



## services\.comin\.ociRemotes\.\*\.layer_media_type



The media type of the artifact layer containing the
configuration\. When null, the first layer with a
tar or compressed tar media type is used\.



*Type:*
null or string



*Default:*

```nix
null
```



*Example:*

```nix
"application/vnd.cncf.flux.content.v1.tar+gzip"
```



## services\.comin\.ociRemotes\.\*\.name



The name of the remote\.



*Type:*
string



## services\.comin\.ociRemotes\.\*\.poller



The poller options\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.ociRemotes\.\*\.poller\.period



The poller period in seconds\.



*Type:*
signed integer



*Default:*

```nix
60
```



## services\.comin\.ociRemotes\.\*\.ref



The references of the artifacts to pull\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.ociRemotes\.\*\.ref\.main



The reference of the main artifact\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.ociRemotes\.\*\.ref\.main\.digest



The digest the main artifact is pinned to\.



*Type:*
null or string



*Default:*

```nix
null
```



*Example:*

```nix
"sha256:3b6cdcc7adcc9a84d3214ee1c029543789d90b5ae69debe9efa3f66e982875de"
```



## services\.comin\.ociRemotes\.\*\.ref\.main\.operation



The switch-to-configuration operation to do on the main artifact\.



*Type:*
one of “switch”, “test”, “boot”



*Default:*

```nix
"switch"
```



## services\.comin\.ociRemotes\.\*\.ref\.main\.semver



A semver constraint\. The highest tag matching this
constraint is selected\. Artifacts with a version lower
than the deployed one are refused\.



*Type:*
null or string



*Default:*

```nix
null
```



*Example:*

```nix
">=1.0.0"
```



## services\.comin\.ociRemotes\.\*\.ref\.main\.semver_filter



A regular expression tags have to match to be considered
by the semver constraint\.



*Type:*
null or string



*Default:*

```nix
null
```



*Example:*

```nix
".*-rc.*"
```



## services\.comin\.ociRemotes\.\*\.ref\.main\.tag



The tag of the main artifact\.



*Type:*
null or string



*Default:*
` latest ` when ` semver ` and ` digest ` are not set



## services\.comin\.ociRemotes\.\*\.ref\.testing



The reference of the testing artifact\. A
testing artifact is preferred when it is newer
than the main artifact\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.ociRemotes\.\*\.ref\.testing\.digest



The digest the testing artifact is pinned to\.



*Type:*
null or string



*Default:*

```nix
null
```



*Example:*

```nix
"sha256:3b6cdcc7adcc9a84d3214ee1c029543789d90b5ae69debe9efa3f66e982875de"
```



## services\.comin\.ociRemotes\.\*\.ref\.testing\.operation



The switch-to-configuration operation to do on the testing artifact\.



*Type:*
one of “switch”, “test”, “boot”



*Default:*

```nix
"test"
```



## services\.comin\.ociRemotes\.\*\.ref\.testing\.semver



A semver constraint\. The highest tag matching this
constraint is selected\. Artifacts with a version lower
than the deployed one are refused\.



*Type:*
null or string



*Default:*

```nix
null
```



*Example:*

```nix
">=1.0.0"
```



## services\.comin\.ociRemotes\.\*\.ref\.testing\.semver_filter



A regular expression tags have to match to be considered
by the semver constraint\.



*Type:*
null or string



*Default:*

```nix
null
```



*Example:*

```nix
".*-rc.*"
```



## services\.comin\.ociRemotes\.\*\.ref\.testing\.tag



The tag of the testing artifact\.



*Type:*
null or string



*Default:*
` testing-${config.services.comin.hostname} ` when ` semver ` and ` digest ` are not set



## services\.comin\.ociRemotes\.\*\.timeout



The fetch timeout in seconds\.



*Type:*
signed integer



*Default:*

```nix
300
```



## services\.comin\.ociRemotes\.\*\.url



The URL of the OCI repository\.



*Type:*
string matching the pattern oci://\.+



*Example:*

```nix
"oci://ghcr.io/owner/infra"
```



## services\.comin\.postBuildCommand



A path to a script executed after each
build (including builds that are already in the Nix store)\.
comin provides to the script the same environment variables as
for the post deployment command, with ` COMIN_PHASE ` set to
` build `\. The ` COMIN_STATUS ` is one of ` built `, ` failed ` or
` already built `\.



*Type:*
null or absolute path



*Default:*

```nix
null
```



*Example:*

```nix
pkgs.writers.writeBash "post-build" "echo $COMIN_GIT_SHA";

```



## services\.comin\.postDeploymentCommand



A path to a script executed after each
deployment\. comin provides to the script the following
environment variables: ` COMIN_PHASE ` (set to ` deploy `),
` COMIN_GIT_SHA `, ` COMIN_GIT_REF `, ` COMIN_GIT_MSG `,
` COMIN_HOSTNAME `, ` COMIN_GENERATION `, ` COMIN_STATUS `
(one of ` init `, ` running `, ` done ` or ` failed `) and
` COMIN_ERROR_MSG `\. When the configuration comes from an OCI
artifact, ` COMIN_GIT_SHA ` and ` COMIN_GIT_REF ` are read from
its ` org.opencontainers.image.revision ` annotation and the
` COMIN_OCI_REMOTE `, ` COMIN_OCI_TAG ` and ` COMIN_OCI_DIGEST `
variables are also provided\.



*Type:*
null or absolute path



*Default:*

```nix
null
```



*Example:*

```nix
pkgs.writers.writeBash "post" "echo $COMIN_GIT_SHA";

```



## services\.comin\.remotes



Ordered list of repositories to pull\.



*Type:*
list of (submodule)



*Default:*

```nix
[ ]
```



## services\.comin\.remotes\.\*\.auth



Authentication options\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.remotes\.\*\.auth\.access_token_path



The path of the auth file\.



*Type:*
string



*Default:*

```nix
""
```



## services\.comin\.remotes\.\*\.auth\.ssh_deploy_key_path



Path to the SSH private key used to authenticate to the Git remote\.



*Type:*
string



*Default:*

```nix
""
```



## services\.comin\.remotes\.\*\.auth\.ssh_known_hosts_path



Path to the known_hosts file used to verify the SSH
host key of the Git remote\. Defaults to
/etc/ssh/ssh_known_hosts when unset\. The remote’s host
key must be present in this file\.



*Type:*
string



*Default:*

```nix
""
```



## services\.comin\.remotes\.\*\.auth\.username



The username used to authenticate to the Git
remote repository\. Note that any non empty
username is valid on GitLab and GitHub\.



*Type:*
string



*Default:*

```nix
"comin"
```



## services\.comin\.remotes\.\*\.branches



Branches to pull\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.remotes\.\*\.branches\.main



The main branch to fetch\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.remotes\.\*\.branches\.main\.name



The name of the main branch\.



*Type:*
string



*Default:*

```nix
"main"
```



## services\.comin\.remotes\.\*\.branches\.main\.operation



The switch-to-configuration operation to do on this branch\.



*Type:*
one of “switch”, “test”, “boot”



*Default:*

```nix
"switch"
```



## services\.comin\.remotes\.\*\.branches\.testing



The testing branch to fetch\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.remotes\.\*\.branches\.testing\.name



The name of the testing branch\.



*Type:*
string



*Default:*

```nix
testing-${config.services.comin.hostname}
```



## services\.comin\.remotes\.\*\.branches\.testing\.operation



The switch-to-configuration operation to do on this branch\.



*Type:*
one of “switch”, “test”, “boot”



*Default:*

```nix
"test"
```



## services\.comin\.remotes\.\*\.name



The name of the remote\.



*Type:*
string



## services\.comin\.remotes\.\*\.poller



The poller options\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.remotes\.\*\.poller\.period



The poller period in seconds\.



*Type:*
signed integer



*Default:*

```nix
60
```



## services\.comin\.remotes\.\*\.timeout



Git fetch timeout in seconds\.



*Type:*
signed integer



*Default:*

```nix
300
```



## services\.comin\.remotes\.\*\.url



The URL of the repository\.



*Type:*
string



## services\.comin\.repositorySubdir



Subdirectory in the repository, containing a default\.nix or a flake\.nix file\.



*Type:*
string



*Default:*

```nix
"."
```



## services\.comin\.repositoryType



The type of the repository to fetch\. It can either contains a flake or a classical Nix expression\.



*Type:*
one of “flake”, “nix”



*Default:*

```nix
"flake"
```



## services\.comin\.retention



The deployments and profiles retention policyes\.



*Type:*
submodule



*Default:*

```nix
{ }
```



## services\.comin\.retention\.deployment_any_capacity



Total number of deployments to keep\. Includes all deployments
regardless of status (including failed deployments)\.



*Type:*
signed integer



*Default:*

```nix
5
```



## services\.comin\.retention\.deployment_boot_entry_capacity



Number of boot entries to keep\. Controls how many successful
deployments generating boot entries (boot or switch operations)
with unique storepaths are retained\.



*Type:*
signed integer



*Default:*

```nix
3
```



## services\.comin\.retention\.deployment_successful_capacity



Number of successful deployments to keep\. Includes all deployments
with status=done, regardless of operation type\.



*Type:*
signed integer



*Default:*

```nix
3
```



## services\.comin\.sshAllowedSignersPath



An OpenSSH allowed signers file path used to verify SSH-signed Git commits\.



*Type:*
null or string



*Default:*

```nix
null
```



## services\.comin\.submodules



Whether to fetch and include Git submodules when cloning the repository\.
When enabled, this adds ?submodules=1 to the flake URL\.



*Type:*
boolean



*Default:*

```nix
false
```



## services\.comin\.systemAttr



This is the attribute containing the machine toplevel
attribute\. Note this is only used when the repositoryType is
‘nix’\. When the repository type is ‘flake’, the attribute is
derived from the hostname\.



*Type:*
null or string



*Default:*

```nix
null
```
