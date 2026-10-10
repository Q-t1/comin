package cmd

import (
	"os"
	"path"
	"time"

	brokerPkg "github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/builder"
	"github.com/nlewo/comin/internal/config"
	"github.com/nlewo/comin/internal/deployer"
	executorPkg "github.com/nlewo/comin/internal/executor"
	"github.com/nlewo/comin/internal/fetcher"
	"github.com/nlewo/comin/internal/http"
	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/oci"
	"github.com/nlewo/comin/internal/prometheus"
	"github.com/nlewo/comin/internal/repository"
	"github.com/nlewo/comin/internal/scheduler"
	"github.com/nlewo/comin/internal/server"
	storePkg "github.com/nlewo/comin/internal/store"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var configFilepath string

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run comin to deploy your published configurations",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Read(configFilepath)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

		gitConfig := config.MkGitConfig(cfg)
		ociConfig := config.MkOCIConfig(cfg)

		if err := os.MkdirAll(cfg.StateDir, os.ModePerm); err != nil {
			logrus.Errorf("Failed to create the state dir %s: %s", cfg.StateDir, err)
			return
		}
		// TODO: this could be removed from release > 0.9.0.
		//
		// Previous comin versions didn't correctly set the
		// permissions of the /var/lib/comin folder. This
		// means we need to explicitly chown them to fix this
		// for existing comin deployments.
		if err := os.Chmod(cfg.StateDir, 0755); err != nil {
			logrus.Errorf("Failed to chmod the state dir %s: %s", cfg.StateDir, err)
			return
		}

		var executor executorPkg.Executor
		executor, err = executorPkg.New(cfg.RepositoryType, gitConfig.Path, ociConfig.Path, gitConfig.Submodules)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

		machineId, err := executor.ReadMachineId()
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

		metrics := prometheus.New()
		storeFilename := path.Join(cfg.StateDir, "store.json")
		gcRootsDir := path.Join(cfg.StateDir, "gcroots")
		broker := brokerPkg.New()
		broker.Start()
		store, err := storePkg.New(broker, storeFilename, gcRootsDir, cfg.Retention.DeploymentBootEntryCapacity, cfg.Retention.DeploymentSuccessfulCapacity, cfg.Retention.DeploymentAnyCapacity)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
		if err := store.Load(); err != nil {
			logrus.Errorf("Ignoring the state file %s because of the loading error: %s", storeFilename, err)
		}
		metrics.SetBuildInfo(cmd.Version)

		var lastDeployment *protobuf.Deployment
		if ok, ld := store.LastDeployment(); ok {
			lastDeployment = ld
			metrics.SetDeploymentInfo(ld.Generation.GetSource().Id(), ld.Status)
		}

		var f fetcher.Fetcher
		if len(cfg.OCIRemotes) > 0 {
			// We get the last main artifact to avoid useless
			// redeployments as well as rollbacks to older artifacts
			var main oci.MainArtifact
			if ociSource := lastDeployment.GetGeneration().GetSource().GetOci(); ociSource != nil {
				main = oci.MainArtifact{
					Digest:     ociSource.MainDigest,
					Tag:        ociSource.MainTag,
					Created:    ociSource.MainCreated,
					RemoteName: ociSource.MainRemoteName,
				}
			}
			repository, err := oci.New(ociConfig, main)
			if err != nil {
				logrus.Errorf("Failed to initialize the OCI repository: %s", err)
				os.Exit(1)
			}
			f = fetcher.NewOCIFetcher(repository, broker)
		} else {
			// We get the last mainCommitId to avoid useless
			// redeployment as well as non fast forward checkouts
			mainCommitId := lastDeployment.GetGeneration().GetSource().GetGit().GetMainCommitId()
			repository, err := repository.New(gitConfig, mainCommitId, metrics)
			if err != nil {
				logrus.Errorf("Failed to initialize the repository: %s", err)
				os.Exit(1)
			}
			f = fetcher.NewGitFetcher(repository, broker)
		}
		f.Start(cmd.Context())
		sched := scheduler.New()
		for _, remote := range cfg.Remotes {
			sched.FetchRemote(f, remote.Name, remote.Poller)
		}
		for _, remote := range cfg.OCIRemotes {
			sched.FetchRemote(f, remote.Name, remote.Poller)
		}

		builder := builder.New(store, executor, broker, gitConfig.Path, gitConfig.Dir, cfg.SystemAttr, cfg.Hostname, gitConfig.Submodules, time.Duration(cfg.EvalTimeout)*time.Second, time.Duration(cfg.BuildTimeout)*time.Second, cfg.BuildAttemptsLimit, time.Duration(cfg.BuildRetryTimer)*time.Second, cfg.PostBuildCommand)
		deployer := deployer.New(store, executor.Deploy, lastDeployment, cfg.PostDeploymentCommand, broker)

		mode, err := manager.ParseMode(cfg.BuildConfirmer.Mode)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
		buildConfirmer := manager.NewConfirmer(broker, mode, time.Duration(cfg.BuildConfirmer.AutoDuration)*time.Second, "build")
		buildConfirmer.Start()
		mode, err = manager.ParseMode(cfg.DeployConfirmer.Mode)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
		deployConfirmer := manager.NewConfirmer(broker, mode, time.Duration(cfg.DeployConfirmer.AutoDuration)*time.Second, "deploy")
		deployConfirmer.Start()

		configurationOperations := manager.ConfigurationOperations{}
		for _, r := range cfg.Remotes {
			configurationOperations[r.Name] = make(map[string]string)
			configurationOperations[r.Name][r.Branches.Main.Name] = r.Branches.Main.Operation
			configurationOperations[r.Name][r.Branches.Testing.Name] = r.Branches.Testing.Operation
		}
		for _, r := range cfg.OCIRemotes {
			configurationOperations[r.Name] = map[string]string{
				manager.OCIRefMain:    r.Ref.Main.Operation,
				manager.OCIRefTesting: r.Ref.Testing.Operation,
			}
		}
		manager := manager.New(store, metrics, sched, f, builder, deployer, machineId, cfg.Hostname, executor, buildConfirmer, deployConfirmer, broker, configurationOperations)

		http.Serve(manager,
			metrics,
			cfg.ApiServer.ListenAddress, cfg.ApiServer.Port,
			cfg.Exporter.ListenAddress, cfg.Exporter.Port)
		srv := server.New(broker, manager, cfg.Grpc.UnixSocketPath)
		srv.Start()

		prometheus.Subscribe(broker, &metrics)

		manager.Run(cmd.Context())
	},
}

func init() {
	runCmd.PersistentFlags().StringVarP(&configFilepath, "config", "", "", "the configuration file path")
	_ = runCmd.MarkPersistentFlagRequired("config")
	rootCmd.AddCommand(runCmd)
}
