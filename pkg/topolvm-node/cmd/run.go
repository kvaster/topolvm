package cmd

import (
	"errors"
	"github.com/spf13/viper"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/controllers"
	"github.com/topolvm/topolvm/csi"
	"github.com/topolvm/topolvm/driver"
	"github.com/topolvm/topolvm/driver/k8s"
	"github.com/topolvm/topolvm/lvm"
	"github.com/topolvm/topolvm/runners"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	if err := topolvmv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		panic(err)
	}

	// +kubebuilder:scaffold:scheme
}

func subMain() error {
	nodename := viper.GetString("nodename")
	if len(nodename) == 0 {
		return errors.New("node name is not given")
	}

	ctrl.SetLogger(zap.New(zap.UseDevMode(config.development)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: config.metricsAddr,
		LeaderElection:     false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	lvmc, err := lvm.New("/pool")
	if err != nil {
		setupLog.Error(err, "unable to create lvm client")
		return err
	}

	lvcontroller := controllers.NewLogicalVolumeReconciler(
		mgr.GetClient(),
		lvmc,
		ctrl.Log.WithName("controllers").WithName("LogicalVolume"),
		nodename,
	)

	if err := lvcontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolume")
		return err
	}
	// +kubebuilder:scaffold:builder

	// Add metrics exporter to manager.
	// Note that grpc.ClientConn can be shared with multiple stubs/services.
	// https://github.com/grpc/grpc-go/tree/master/examples/features/multiplex
	if err := mgr.Add(runners.NewMetricsExporter(mgr, lvmc, nodename)); err != nil {
		return err
	}

	// Add gRPC server to manager.
	s, err := k8s.NewLogicalVolumeService(mgr)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	csi.RegisterNodeServer(grpcServer, driver.NewNodeService(nodename, lvmc, s))
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
