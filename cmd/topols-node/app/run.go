package app

import (
	"context"
	"errors"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kvaster/topols"
	topolsv1 "github.com/kvaster/topols/api/v1"
	clientwrapper "github.com/kvaster/topols/internal/client"
	"github.com/kvaster/topols/internal/controller"
	"github.com/kvaster/topols/internal/driver"
	"github.com/kvaster/topols/internal/lsm"
	"github.com/kvaster/topols/internal/runners"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(topolsv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func subMain() error {
	nodename := viper.GetString("nodename")
	if len(nodename) == 0 {
		return errors.New("node name is not given")
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&config.zapOpts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: config.metricsAddr,
		},
		LeaderElection: false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}
	reader := clientwrapper.NewWrappedClient(mgr.GetClient())
	apiReader := clientwrapper.NewWrappedReader(mgr.GetAPIReader(), mgr.GetClient().Scheme())

	lsmc, err := lsm.New(config.poolPath)
	if err != nil {
		setupLog.Error(err, "unable to create ls client")
		return err
	}
	if err := mgr.Add(lsmc); err != nil {
		return err
	}

	lvcontroller := controller.NewLogicalVolumeReconciler(reader, lsmc, nodename)
	if err := lvcontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolume")
		return err
	}
	//+kubebuilder:scaffold:builder

	// Add health checker to manager
	checker := runners.NewChecker(checkFunc(apiReader), 1*time.Minute)
	if err := mgr.Add(checker); err != nil {
		return err
	}

	// Add metrics exporter to manager.
	// Note that grpc.ClientConn can be shared with multiple stubs/services.
	// https://github.com/grpc/grpc-go/tree/master/examples/features/multiplex
	if err := mgr.Add(runners.NewMetricsExporter(reader, lsmc, nodename)); err != nil {
		return err
	}

	// Add gRPC server to manager.
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(ErrorLoggingInterceptor))
	csi.RegisterIdentityServer(grpcServer, driver.NewIdentityServer(checker.Ready))
	nodeServer, err := driver.NewNodeServer(nodename, lsmc, mgr)
	if err != nil {
		return err
	}
	csi.RegisterNodeServer(grpcServer, nodeServer)
	err = mgr.Add(runners.NewGRPCRunner(grpcServer, config.csiSocket, false))
	if err != nil {
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch

func checkFunc(r client.Reader) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var drv storagev1.CSIDriver
		return r.Get(ctx, types.NamespacedName{Name: topols.PluginName}, &drv)
	}
}

func ErrorLoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	resp, err = handler(ctx, req)
	if err != nil {
		ctrl.Log.Error(err, "error on grpc call", "method", info.FullMethod)
	}
	return resp, err
}
