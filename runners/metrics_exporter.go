package runners

import (
	"context"
	"github.com/topolvm/topolvm/lvm"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const metricsNamespace = "topolvm"

var meLogger = ctrl.Log.WithName("runners").WithName("metrics_exporter")

type metricsExporter struct {
	client.Client
	nodeName       string
	availableBytes *prometheus.GaugeVec
	lvmc           lvm.Client
}

var _ manager.LeaderElectionRunnable = &metricsExporter{}

// NewMetricsExporter creates controller-runtime's manager.Runnable to run
// a metrics exporter for a node.
func NewMetricsExporter(mgr manager.Manager, lvmc lvm.Client, nodeName string) manager.Runnable {
	availableBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volumegroup",
		Name:        "available_bytes",
		Help:        "LVM VG available bytes under lvmd management",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})
	metrics.Registry.MustRegister(availableBytes)

	return &metricsExporter{
		Client:         mgr.GetClient(),
		nodeName:       nodeName,
		availableBytes: availableBytes,
		lvmc:			lvmc,
	}
}

// Start implements controller-runtime's manager.Runnable.
func (m *metricsExporter) Start(ch <-chan struct{}) error {
	metricsCh := make(chan *lvm.DeviceClassStats)
	go func() {
		for {
			select {
			case <-ch:
				return
			case met := <-metricsCh:
				m.availableBytes.WithLabelValues(met.DeviceClass).Set(float64(met.TotalBytes - met.UsedBytes))
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ch
		cancel()
	}()

	// make first update as soon as we start
	// cause node Finalizer is updated here and it's not good...
	// probably this should be fixed somehow
	if err := m.updateNode(ctx, metricsCh); err != nil {
		return err
	}

	watch := m.lvmc.Watch()

	ticker := time.NewTicker(10 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return nil
		case <-watch:
			if err := m.updateNode(ctx, metricsCh); err != nil {
				ticker.Stop()
				return err
			}
		case <-ticker.C:
			if err := m.updateNode(ctx, metricsCh); err != nil {
				ticker.Stop()
				return err
			}
		}
	}
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (m *metricsExporter) NeedLeaderElection() bool {
	return false
}

func (m *metricsExporter) updateNode(ctx context.Context, ch chan<- *lvm.DeviceClassStats) error {
	stats, err := m.lvmc.NodeStats()

	if err != nil {
		return err
	}

	for _, s := range stats.DeviceClasses {
		ch <- s
	}

	var node corev1.Node
	if err := m.Get(ctx, types.NamespacedName{Name: m.nodeName}, &node); err != nil {
		return err
	}

	if node.DeletionTimestamp != nil {
		meLogger.Info("node is deleting")
		return nil
	}

	node2 := node.DeepCopy()

	var hasFinalizer bool
	for _, fin := range node.Finalizers {
		if fin == topolvm.NodeFinalizer {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		node2.Finalizers = append(node2.Finalizers, topolvm.NodeFinalizer)
	}

	if stats.Default != nil {
		node2.Annotations[topolvm.CapacityKeyPrefix+topolvm.DefaultDeviceClassAnnotationName] = strconv.FormatUint(stats.Default.TotalBytes - stats.Default.UsedBytes, 10)
	}

	for _, s := range stats.DeviceClasses {
		node2.Annotations[topolvm.CapacityKeyPrefix+s.DeviceClass] = strconv.FormatUint(s.TotalBytes - s.UsedBytes, 10)
	}
	if err := m.Patch(ctx, node2, client.MergeFrom(&node)); err != nil {
		return err
	}

	return nil
}
