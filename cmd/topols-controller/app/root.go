package app

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/kvaster/topols"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var config struct {
	csiSocket                   string
	metricsAddr                 string
	secureMetricsServer         bool
	healthAddr                  string
	enableWebhooks              bool
	webhookAddr                 string
	certDir                     string
	leaderElection              bool
	leaderElectionID            string
	leaderElectionNamespace     string
	leaderElectionLeaseDuration time.Duration
	leaderElectionRenewDeadline time.Duration
	leaderElectionRetryPeriod   time.Duration
	skipNodeFinalize            bool
	zapOpts                     zap.Options
}

var rootCmd = &cobra.Command{
	Use:     "topols-controller",
	Version: topols.Version,
	Short:   "TopoLS CSI controller",
	Long: `topols-controller provides CSI controller service.
It also works as a custom Kubernetes controller.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	fs := rootCmd.Flags()
	fs.StringVar(&config.csiSocket, "csi-socket", topols.DefaultCSISocket, "UNIX domain socket filename for CSI")
	fs.StringVar(&config.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	fs.BoolVar(&config.secureMetricsServer, "secure-metrics-server", false, "Secures the metrics server")
	fs.StringVar(&config.healthAddr, "health-probe-bind-address", ":8081", "The TCP address that the controller should bind to for serving health probes.")
	fs.StringVar(&config.webhookAddr, "webhook-addr", ":9443", "Listen address for the webhook endpoint")
	fs.BoolVar(&config.enableWebhooks, "enable-webhooks", true, "Enable webhooks")
	fs.StringVar(&config.certDir, "cert-dir", "", "certificate directory")
	fs.BoolVar(&config.leaderElection, "leader-election", true, "Enables leader election. This field is required to be set to true if concurrency is greater than 1 at any given point in time during rollouts.")
	fs.StringVar(&config.leaderElectionID, "leader-election-id", "topols", "ID for leader election by controller-runtime")
	fs.StringVar(&config.leaderElectionNamespace, "leader-election-namespace", "", "Namespace where the leader election resource lives. Defaults to the pod namespace if not set.")
	fs.DurationVar(&config.leaderElectionLeaseDuration, "leader-election-lease-duration", 15*time.Second, "Duration that non-leader candidates will wait to force acquire leadership. This is measured against time of last observed ack.")
	fs.DurationVar(&config.leaderElectionRenewDeadline, "leader-election-renew-deadline", 10*time.Second, "Duration that the acting controlplane will retry refreshing leadership before giving up. This is measured against time of last observed ack.")
	fs.DurationVar(&config.leaderElectionRetryPeriod, "leader-election-retry-period", 2*time.Second, "Duration the LeaderElector clients should wait between tries of actions.")
	fs.BoolVar(&config.skipNodeFinalize, "skip-node-finalize", false, "skips automatic cleanup of PhysicalVolumeClaims when a Node is deleted")

	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	config.zapOpts.BindFlags(goflags)

	fs.AddGoFlagSet(goflags)
}
