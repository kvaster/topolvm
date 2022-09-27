package topols

import corev1 "k8s.io/api/core/v1"

// CapacityKeyPrefix is the key prefix of Node annotation that represents VG free space.
const CapacityKeyPrefix = "capacity.topols.kvaster.com/"

// CapacityResource is the resource name of topols capacity.
const CapacityResource = corev1.ResourceName("topols.kvaster.com/capacity")

// PluginName is the name of the CSI plugin.
const PluginName = "topols.kvaster.com"

// TopologyNodeKey is the key of topology that represents node name.
const TopologyNodeKey = "topology.topols.kvaster.com/node"

// DeviceClassKey is the key used in CSI volume create requests to specify a device-class.
const DeviceClassKey = "topols.kvaster.com/device-class"

// ResizeRequestedAtKey is the key of LogicalVolume that represents the timestamp of the resize request.
const ResizeRequestedAtKey = "topols.kvaster.com/resize-requested-at"

// LogicalVolumeFinalizer is the name of LogicalVolume finalizer
const LogicalVolumeFinalizer = "topols.kvaster.com/logicalvolume"

// NodeFinalizer is the name of Node finalizer of TopoLS
const NodeFinalizer = "topols.kvaster.com/node"

// PVCFinalizer is the name of PVC finalizer of TopoLS
const PVCFinalizer = "topols.kvaster.com/pvc"

// DefaultCSISocket is the default path of the CSI socket file.
const DefaultCSISocket = "/run/topols/csi-topols.sock"

// DefaultDeviceClassAnnotationName is the part of annotation name for the default device-class.
const DefaultDeviceClassAnnotationName = "00default"

// DefaultDeviceClassName is the name for the default device-class.
const DefaultDeviceClassName = ""

// DefaultDeviceClassKey is the key that represents default device class on Node
const DefaultDeviceClassKey = "topols.kvaster.com/default-device-class"

// DefaultSizeGb is the default size in GiB for  volumes (PVC or generic ephemeral volumes) w/o capacity requests.
const DefaultSizeGb = 1

// DefaultSize is DefaultSizeGb in bytes
const DefaultSize = DefaultSizeGb << 30

// Label key that indicates The controller/user who created this resource
// https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels
const CreatedbyLabelKey = "app.kubernetes.io/created-by"

// Label value that indicates The controller/user who created this resource
const CreatedbyLabelValue = "topols-controller"
