# TopoLS Helm Chart
----------------------------------------

## Prerequisites

* Configure `kube-scheduler` on the underlying nodes, ref: https://github.com/kvaster/topols/tree/master/deploy#configure-kube-scheduler
* `cert-manager` version `v1.7.0+` installed. ref: https://cert-manager.io/
* Requires at least `v3.5.0+` version of helm to support

## :warning: Migration from kustomize to Helm

See [MIGRATION.md](./MIGRATION.md)

## How to use TopoLS Helm repository

You need to add this repository to your Helm repositories:

```sh
helm repo add topols https://kvaster.github.io/topols
helm repo update
```

## Dependencies

| Repository | Name	| Version |
| ---------- | ---- | ------- |
| https://charts.jetstack.io | cert-manager | 1.7.0 |

## Quick start

By default, the [topols-scheduler](../../deploy/README.md#topols-scheduler) runs in a DaemonSet.
It can alternatively run inside a Deployment.

### Installing the Chart

> :memo: NOTE: This installation method requires cert-manger to be installed beforehand.

To work webhooks properly, add a label to the target namespace. We also recommend to use a dedicated namespace.

```sh
kubectl label namespace topols-system topols.kvaster.com/webhook=ignore
kubectl label namespace kube-system topols.kvaster.com/webhook=ignore
```

Install the chart with the release name `topols` into the namespace:

```sh
helm install --namespace=topols-system topols topols/topols
```

Specify parameters using `--set key=value[,key=value]` argument to `helm install`.

Alternatively a YAML file that specifies the values for the parameters can be provided like this:

```sh
helm upgrade --namespace=topols-system -f values.yaml -i topols topols/topols
```

### Install together with cert-manager

Before installing the chart, you must first install the cert-manager CustomResourceDefinition resources.

```sh
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.7.0/cert-manager.crds.yaml
```

Set the `cert-manager.enabled=true` parameter when installing topols chart:

```sh
helm install --namespace=topols-system topols topols/topols --set cert-manager.enabled=true
```

## Configure kube-scheduler

The current Chart does not provide an option to make kube-scheduler configurable.
You need to configure kube-scheduler to use topols-scheduler extender by referring to the following document.

[deploy/README.md#configure-kube-scheduler](../../deploy/README.md#configure-kube-scheduler)

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| cert-manager.enabled | bool | `false` | Install cert-manager together. # ref: https://cert-manager.io/docs/installation/kubernetes/#installing-with-helm |
| controller.affinity | object | `{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchExpressions":[{"key":"app.kubernetes.io/component","operator":"In","values":["controller"]},{"key":"app.kubernetes.io/name","operator":"In","values":["{{ include \"topols.name\" . }}"]}]},"topologyKey":"kubernetes.io/hostname"}]}}` | Specify affinity. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity |
| controller.args | list | `[]` | Arguments to be passed to the command. |
| controller.minReadySeconds | int | `nil` | Specify minReadySeconds. |
| controller.nodeFinalize.skipped | bool | `false` | Skip automatic cleanup of PhysicalVolumeClaims when a Node is deleted. |
| controller.nodeSelector | object | `{}` | Specify nodeSelector. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ |
| controller.podDisruptionBudget.enabled | bool | `true` | Specify podDisruptionBudget enabled. |
| controller.priorityClassName | string | `nil` | Specify priorityClassName. |
| controller.prometheus.podMonitor.additionalLabels | object | `{}` | Additional labels that can be used so PodMonitor will be discovered by Prometheus. |
| controller.prometheus.podMonitor.enabled | bool | `false` | Set this to `true` to create PodMonitor for Prometheus operator. |
| controller.prometheus.podMonitor.interval | string | `""` | Scrape interval. If not set, the Prometheus default scrape interval is used. |
| controller.prometheus.podMonitor.metricRelabelings | list | `[]` | MetricRelabelConfigs to apply to samples before ingestion. |
| controller.prometheus.podMonitor.namespace | string | `""` | Optional namespace in which to create PodMonitor. |
| controller.prometheus.podMonitor.relabelings | list | `[]` | RelabelConfigs to apply to samples before scraping. |
| controller.prometheus.podMonitor.scrapeTimeout | string | `""` | Scrape timeout. If not set, the Prometheus default scrape timeout is used. |
| controller.replicaCount | int | `2` | Number of replicas for CSI controller service. |
| controller.securityContext.enabled | bool | `true` | Enable securityContext. |
| controller.storageCapacityTracking.enabled | bool | `false` | Enable Storage Capacity Tracking for csi-provisioner. |
| controller.terminationGracePeriodSeconds | int | `nil` | Specify terminationGracePeriodSeconds. |
| controller.tolerations | list | `[]` | Specify tolerations. # ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| controller.updateStrategy | object | `{}` | Specify updateStrategy. |
| controller.volumes | list | `[{"emptyDir":{},"name":"socket-dir"}]` | Specify volumes. |
| image.csi.csiProvisioner | string | `nil` | Specify csi-provisioner image. If not specified, `ghcr.io/kvaster/topols-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.csi.csiResizer | string | `nil` | Specify csi-resizer image. If not specified, `ghcr.io/kvaster/topols-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.csi.csiSnapshotter | string | `nil` | Specify csi-snapshot image. If not specified, `ghcr.io/topolvm/topolvm-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.csi.livenessProbe | string | `nil` | Specify livenessprobe image. If not specified, `ghcr.io/kvaster/topols-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.csi.nodeDriverRegistrar | string | `nil` | Specify csi-node-driver-registrar: image. If not specified, `ghcr.io/kvaster/topols-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.pullPolicy | string | `nil` | TopoLS image pullPolicy. |
| image.pullSecrets | list | `[]` | List of imagePullSecrets. |
| image.repository | string | `"ghcr.io/kvaster/topols-with-sidecar"` | TopoLS |
| image.tag | string | `{{ .Chart.AppVersion }}` | TopoLS image tag to use. |
| node.args | list | `[]` | Arguments to be passed to the command. |
| node.kubeletWorkDirectory | string | `"/var/lib/kubelet"` | Specify the work directory of Kubelet on the host. For example, on microk8s it needs to be set to `/var/snap/microk8s/common/var/lib/kubelet` |
| node.metrics.annotations | object | `{"prometheus.io/port":"metrics"}` | Annotations for Scrape used by Prometheus. |
| node.metrics.enabled | bool | `true` | If true, enable scraping of metrics by Prometheus. |
| node.nodeSelector | object | `{}` | Specify nodeSelector. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ |
| node.poolPath | string | `"/mnt/pool"` |  |
| node.priorityClassName | string | `nil` | Specify priorityClassName. |
| node.prometheus.podMonitor.additionalLabels | object | `{}` | Additional labels that can be used so PodMonitor will be discovered by Prometheus. |
| node.prometheus.podMonitor.enabled | bool | `false` | Set this to `true` to create PodMonitor for Prometheus operator. |
| node.prometheus.podMonitor.interval | string | `""` | Scrape interval. If not set, the Prometheus default scrape interval is used. |
| node.prometheus.podMonitor.metricRelabelings | list | `[]` | MetricRelabelConfigs to apply to samples before ingestion. |
| node.prometheus.podMonitor.namespace | string | `""` | Optional namespace in which to create PodMonitor. |
| node.prometheus.podMonitor.relabelings | list | `[]` | RelabelConfigs to apply to samples before scraping. |
| node.prometheus.podMonitor.scrapeTimeout | string | `""` | Scrape timeout. If not set, the Prometheus default scrape timeout is used. |
| node.psp.allowedHostPaths | list | `[]` | Specify allowedHostPaths. |
| node.securityContext.capabilities.add[0] | string | `"SYS_ADMIN"` |  |
| node.securityContext.privileged | bool | `true` |  |
| node.tolerations | list | `[]` | Specify tolerations. # ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| node.updateStrategy | object | `{}` | Specify updateStrategy. |
| node.volumeMounts.topolsNode | list | `[]` | Specify volumeMounts for topols-node container. |
| node.volumes | list | `[]` | Specify volumes. |
| podSecurityPolicy.create | bool | `true` | Enable pod security policy. # ref: https://kubernetes.io/docs/concepts/policy/pod-security-policy/ |
| priorityClass.enabled | bool | `true` | Install priorityClass. |
| priorityClass.name | string | `"topols"` | Specify priorityClass resource name. |
| priorityClass.value | int | `1000000` |  |
| resources.csi_provisioner | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.csi_registrar | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.csi_resizer | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.csi_snapshotter | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.liveness_probe | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.topols_controller | object | `{}` |  |
| resources.topols_node | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.topols_scheduler | object | `{}` |  |
| scheduler.affinity | object | `{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"node-role.kubernetes.io/control-plane","operator":"Exists"}]},{"matchExpressions":[{"key":"node-role.kubernetes.io/master","operator":"Exists"}]}]}}}` | Specify affinity on the Deployment or DaemonSet. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity |
| scheduler.args | list | `[]` | Arguments to be passed to the command. |
| scheduler.deployment.replicaCount | int | `2` | Number of replicas for Deployment. |
| scheduler.enabled | bool | `true` | If true, enable scheduler extender for TopoLS |
| scheduler.minReadySeconds | int | `nil` | Specify minReadySeconds on the Deployment or DaemonSet. |
| scheduler.nodeSelector | object | `{}` | Specify nodeSelector on the Deployment or DaemonSet. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ |
| scheduler.options.listen.host | string | `"localhost"` | Host used by Probe. |
| scheduler.options.listen.port | int | `9251` | Listen port. |
| scheduler.podDisruptionBudget.enabled | bool | `true` | Specify podDisruptionBudget enabled. |
| scheduler.priorityClassName | string | `nil` | Specify priorityClassName on the Deployment or DaemonSet. |
| scheduler.schedulerOptions | object | `{}` | Tune the Node scoring. ref: https://github.com/kvaster/topols/blob/master/deploy/README.md |
| scheduler.service.clusterIP | string | `nil` | Specify Service clusterIP. |
| scheduler.service.nodePort | int | `nil` | Specify nodePort. |
| scheduler.service.type | string | `"LoadBalancer"` | Specify Service type. |
| scheduler.terminationGracePeriodSeconds | int | `nil` | Specify terminationGracePeriodSeconds on the Deployment or DaemonSet. |
| scheduler.tolerations | list | `[{"key":"CriticalAddonsOnly","operator":"Exists"},{"effect":"NoSchedule","key":"node-role.kubernetes.io/control-plane"},{"effect":"NoSchedule","key":"node-role.kubernetes.io/master"}]` | Specify tolerations on the Deployment or DaemonSet. # ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| scheduler.type | string | `"daemonset"` | If you run with a managed control plane (such as GKE, AKS, etc), topols-scheduler should be deployed as Deployment and Service. topols-scheduler should otherwise be deployed as DaemonSet in unmanaged (i.e. bare metal) deployments. possible values:  daemonset/deployment |
| scheduler.updateStrategy | object | `{}` | Specify updateStrategy on the Deployment or DaemonSet. |
| securityContext.runAsGroup | int | `10000` | Specify runAsGroup. |
| securityContext.runAsUser | int | `10000` | Specify runAsUser. |
| snapshot.enabled | bool | `true` | Turn on the snapshot feature. |
| storageClasses | list | `[{"name":"topols","storageClass":{"additionalParameters":{},"allowVolumeExpansion":true,"annotations":{},"isDefaultClass":false,"reclaimPolicy":null,"volumeBindingMode":"WaitForFirstConsumer"}}]` | Whether to create storageclass(es) ref: https://kubernetes.io/docs/concepts/storage/storage-classes/ |
| webhook.caBundle | string | `nil` | Specify the certificate to be used for AdmissionWebhook. |
| webhook.existingCertManagerIssuer | object | `{}` | Specify the cert-manager issuer to be used for AdmissionWebhook. |
| webhook.podMutatingWebhook.enabled | bool | `true` | Enable Pod MutatingWebhook. |
| webhook.pvcMutatingWebhook.enabled | bool | `true` | Enable PVC MutatingWebhook. |

## Generate Manifests

You can use the `helm template` command to render manifests.

```sh
helm template --include-crds --namespace=topols-system topols topols/topols
```

## Update README

The `README.md` for this chart is generated by [helm-docs](https://github.com/norwoodj/helm-docs).
To update the README, edit the `README.md.gotmpl` file and run the helm-docs command.

```console
# path to topols repository root
$ make setup
$ ./bin/helm-docs
INFO[2021-06-13T21:43:55+09:00] Found Chart directories [charts/topols]
INFO[2021-06-13T21:43:55+09:00] Generating README Documentation for chart /path/to/dir/topols/topols/charts/topols
```

## Release Chart

See [RELEASE.md](../../RELEASE.md)
