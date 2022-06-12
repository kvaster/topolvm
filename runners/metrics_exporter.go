package runners

import (
	"context"
	"github.com/kvaster/topols/lsm"
	"strconv"
	"strings"
	"time"

	"github.com/kvaster/topols"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const metricsNamespace = "topols"

var meLogger = ctrl.Log.WithName("runners").WithName("metrics_exporter")

type metricsExporter struct {
	client         client.Client
	nodeName       string
	availableBytes *prometheus.GaugeVec
	sizeBytes      *prometheus.GaugeVec
	lvmc           lsm.Client
}

var _ manager.LeaderElectionRunnable = &metricsExporter{}

// NewMetricsExporter creates controller-runtime's manager.Runnable to run
// a metrics exporter for a node.
func NewMetricsExporter(mgr manager.Manager, lvmc lsm.Client, nodeName string) manager.Runnable {
	availableBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volumegroup",
		Name:        "available_bytes",
		Help:        "local storage available bytes under topols management",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})
	metrics.Registry.MustRegister(availableBytes)

	sizeBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volumegroup",
		Name:        "size_bytes",
		Help:        "local storage size bytes under topols management",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_class"})
	metrics.Registry.MustRegister(sizeBytes)

	return &metricsExporter{
		client:         mgr.GetClient(),
		nodeName:       nodeName,
		availableBytes: availableBytes,
		sizeBytes:      sizeBytes,
		lvmc:           lvmc,
	}
}

// Start implements controller-runtime's manager.Runnable.
func (m *metricsExporter) Start(ctx context.Context) error {
	metricsCh := make(chan *lsm.DeviceClassStats)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case met := <-metricsCh:
				m.availableBytes.WithLabelValues(met.DeviceClass).Set(float64(met.TotalBytes - met.UsedBytes))
				m.sizeBytes.WithLabelValues(met.DeviceClass).Set(float64(met.TotalBytes))
			}
		}
	}()

	watch := m.lvmc.Watch()

	// make first update as soon as we start
	// cause node Finalizer is updated here and it's not good...
	// probably this should be fixed somehow
	if err := m.updateNode(ctx, metricsCh); err != nil {
		return err
	}

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

func (m *metricsExporter) updateNode(ctx context.Context, ch chan<- *lsm.DeviceClassStats) error {
	stats, err := m.lvmc.NodeStats()

	if err != nil {
		return err
	}

	for _, s := range stats.DeviceClasses {
		ch <- s
	}

	var node corev1.Node
	if err := m.client.Get(ctx, types.NamespacedName{Name: m.nodeName}, &node); err != nil {
		return err
	}

	if node.DeletionTimestamp != nil {
		meLogger.Info("node is deleting")
		return nil
	}

	node2 := node.DeepCopy()

	var hasFinalizer bool
	for _, fin := range node.Finalizers {
		if fin == topols.NodeFinalizer {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		node2.Finalizers = append(node2.Finalizers, topols.NodeFinalizer)
	}

	if stats.Default != nil {
		node2.Annotations[topols.DefaultDeviceClassKey] = stats.Default.DeviceClass
	} else {
		delete(node2.Annotations, topols.DefaultDeviceClassKey)
	}

	capacityKeys := make(map[string]struct{})
	for k := range node2.Annotations {
		if strings.HasPrefix(k, topols.CapacityKeyPrefix) {
			capacityKeys[k] = struct{}{}
		}
	}

	for _, s := range stats.DeviceClasses {
		key := topols.CapacityKeyPrefix + s.DeviceClass
		node2.Annotations[key] = strconv.FormatUint(s.TotalBytes-s.UsedBytes, 10)
		delete(capacityKeys, key)
	}

	for k := range capacityKeys {
		delete(node2.Annotations, k)
	}

	if err := m.client.Patch(ctx, node2, client.MergeFrom(&node)); err != nil {
		return err
	}

	return nil
}
