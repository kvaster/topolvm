package cmd

import (
	"context"
	"errors"
	"github.com/kvaster/topols"
	topolsv1 "github.com/kvaster/topols/api/v1"
	"github.com/kvaster/topols/controllers"
	"github.com/kvaster/topols/csi"
	"github.com/kvaster/topols/driver"
	"github.com/kvaster/topols/driver/k8s"
	"github.com/kvaster/topols/lsm"
	"github.com/kvaster/topols/runners"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	if err := topolsv1.AddToScheme(scheme); err != nil {
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

	lvmc, err := lsm.New("/mnt/pool")
	if err != nil {
		setupLog.Error(err, "unable to create lvm client")
		return err
	}
	if err := mgr.Add(lvmc); err != nil {
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

	// Add health checker to manager
	checker := runners.NewChecker(checkFunc(mgr.GetAPIReader()), 1*time.Minute)
	if err := mgr.Add(checker); err != nil {
		return err
	}

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
	csi.RegisterIdentityServer(grpcServer, driver.NewIdentityService(checker.Ready))
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

func checkFunc(r client.Reader) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var drv storagev1beta1.CSIDriver
		return r.Get(ctx, types.NamespacedName{Name: topols.PluginName}, &drv)
	}
}
