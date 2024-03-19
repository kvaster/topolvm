package controller

import (
	internalController "github.com/kvaster/topols/internal/controller"
	"github.com/kvaster/topols/pkg/lsm"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupLogicalVolumeReconcilerWithServices creates LogicalVolumeReconciler and sets up with manager.
func SetupLogicalVolumeReconcilerWithServices(mgr ctrl.Manager, client client.Client, lvmc lsm.Client, nodeName string) error {
	reconciler := internalController.NewLogicalVolumeReconciler(client, lvmc, nodeName)
	return reconciler.SetupWithManager(mgr)
}
