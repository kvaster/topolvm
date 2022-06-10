# Deploying TopoLS

Each of these steps are shown in depth in the following sections:

1. Prepare [cert-manager][] for topols-controller. You may supplement an existing instance.
1. Add `topols.kvaster.com/webhook: ignore` label to system namespaces such as `kube-system`.
1. Determine how topols-scheduler to be run:
   - If you run with a managed control plane (such as GKE, AKS, etc), `topols-scheduler` should be deployed as Deployment and Service
   - `topols-scheduler` should otherwise be deployed as DaemonSet in unmanaged (i.e. bare metal) deployments
1. Prepare StorageClasses for TopoLS.
1. Install TopoLS using helm as appropriate to your installation.
1. Configure `kube-scheduler` to use `topols-scheduler`.
1. Configure available storage on each node.

Example configuration files are included in the following sub directories:

- `scheduler-config/`: Configurations to extend `kube-scheduler` with `topols-scheduler`.
- `pool-config`: Configurations of available storage on each node. 

These configuration files may need to be modified for your environment.
Read carefully the following descriptions.

## cert-manager

[cert-manager][] is used to issue self-signed TLS certificate for [topols-controller][].
Follow the [documentation](https://docs.cert-manager.io/en/latest/getting-started/install/kubernetes.html) to install it into your Kubernetes cluster.

### OPTIONAL: Install cert-manager with Helm Chart

Before installing the chart, you must first install the cert-manager CustomResourceDefinition resources.

```sh
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.crds.yaml
```

Set the `cert-manager.enabled=true` in the Helm Chart values.

```yaml
cert-manager:
  enabled: true
```

### OPTIONAL: Prepare the certificate without cert-manager

You can prepare the certificate manually without `cert-manager`.
When doing so, do not apply [certificates.yaml](./manifests/base/certificates.yaml).

1. Prepare PEM encoded self-signed certificate and key files.  
   The certificate must be valid for hostname `controller.topols-system.svc`.
2. Base64-encode the CA cert (in its PEM format)
3. Create Secret in `topols-system` namespace as follows:

    ```console
    kubectl -n topols-system create secret tls mutatingwebhook \
        --cert=<CERTIFICATE FILE> --key=<KEY FILE>
    ```

4. Specify the certificate in the Helm Chart values.

    ```yaml
    <snip>
    webhook:
      caBundle: ... # Base64-encoded, PEM-encoded CA certificate that signs the server certificate
    <snip>
    ```

## Protect system namespaces from TopoLS webhook

TopoLS installs a mutating webhook for Pods.  It may prevent Kubernetes from bootstrapping
if the webhook pods and the system pods are both missing.

To workaround the problem, add a label to system namespaces such as `kube-system` as follows:

```console
$ kubectl label ns kube-system topols.kvaster.com/webhook=ignore
```

## Scheduling

### topols-scheduler

topols-scheduler is a [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for `kube-scheduler`.
It must be deployed to where `kube-scheduler` can connect.

If your Kubernetes cluster runs the control plane on Nodes, `topols-scheduler` should be run as DaemonSet
limited to the control plane nodes. `kube-scheduler` then connects to the extender via loopback network device.

Otherwise, `topols-scheduler` should be run as Deployment and Service.
`kube-scheduler` then connects to the Service address.

#### Running topols-scheduler using DaemonSet

Set the `scheduler.type=daemonset` in the Helm Chart values.
The default is daemonset.

    ```yaml
    <snip>
    scheduler:
      type: daemonset
    <snip>
    ```

#### Running topols-scheduler using Deployment and Service

In this case, you can set the `scheduler.type=deployment` in the Helm Chart values.

    ```yaml
    <snip>
    scheduler:
      type: deployment
    <snip>
    ```

This way, `topols-scheduler` is exposed by LoadBalancer service.

Then edit `urlPrefix` in [scheduler-config.yaml](./scheduler-config/scheduler-config.yaml) for K8s 1.19 or later, to specify the LoadBalancer address.

#### OPTIONAL: tune the node scoring

The node scoring for Pod scheduling can be fine-tuned with the following two ways:
1. Adjust `weights` parameters in the scoring expression
2. Change the weight for the node scoring against the default by kube-scheduler

The scoring expression in `topols-scheduler` is as follows:
```
avg_by_weight((1 - requested / capacity) * 10)
```
For example, if a node has the two different type of disks - small sdd for fast data and big hdd for big data,
`topols-scheduler` can be adjusted for ssd score to have more weight, cause balanced ssd usage is more important
due to smaller capacity. Weight can be specified for each device-class in the Helm Chart values:

```yaml
<snip>
scheduler:
  schedulerOptions:
    weights:
      ssd: 10
      hdd: 1
<snip>
```

Besides, the scoring weight can be passed to kube-scheduler via [scheduler-config.yaml](./scheduler-config/scheduler-config.yaml). Almost all scoring algorithms in kube-scheduler are weighted as `"weight": 1`. So if you want to give a priority to the scoring by `topols-scheduler`, you have to set the weight as a value larger than one like as follows:
```yaml
apiVersion: kubescheduler.config.k8s.io/v1beta1
kind: KubeSchedulerConfiguration
leaderElection:
   leaderElect: true
clientConnection:
   kubeconfig: /etc/kubernetes/scheduler.conf
extenders:
   - urlPrefix: http://127.0.0.1:9251
     filterVerb: predicate
     prioritizeVerb: prioritize
     nodeCacheCapable: false
     weight: 100    # EDIT THIS FIELD #
     managedResources:
        - name: topols.kvaster.com/capacity
          ignoredByScheduler: true
```

### Storage Capacity Tracking

topolvm supports [Storage Capacity Tracking](https://kubernetes.io/docs/concepts/storage/storage-capacity/).
You can enable Storage Capacity Tracking mode instead of using topols-scheduler.
You need to use Kubernetes Cluster v1.21 or later when using Storage Capacity Tracking with topols.

You can see the limitations of using Storage Capacity Tracking from [here](https://kubernetes.io/docs/concepts/storage/storage-capacity/#scheduling).

#### Use Storage Capacity Tracking

If you want to use Storage Capacity Tracking instead of using topols-scheduler,
you need to set the `controller.storageCapacityTracking.enabled=true`, `scheduler.enabled=false` and `webhook.podMutatingWebhook.enabled=false` in the Helm Chart values.

    ```yaml
    <snip>
    controller:
      storageCapacityTracking:
        enabled: true
    <snip>
    scheduler:
      enabled: false
    <snip>
    webhook:
      podMutatingWebhook:
        enabled: false
    <snip>
    ```

## Configure StorageClasses

You need to create [StorageClasses](https://kubernetes.io/docs/concepts/storage/storage-classes/) for TopoLS.
The Helm chart creates a StorageClasses by default with the following configuration.
You can edit the Helm Chart values as needed.

   ```yaml
   <snip>
   storageClasses:
     - name: topols-provisioner
       storageClass:
         isDefaultClass: false
         volumeBindingMode: WaitForFirstConsumer
         allowVolumeExpansion: true
   <snip>
   ```

## Install Helm Chart

The first step is to create a namespace and add a label.

```console
$ kubectl create namespace topols-system
$ kubectl label namespace topols-system topols.kvaster.com/webhook=ignore
```

> :memo: Helm does not support adding labels or other metadata when creating namespaces.
>
> refs: https://github.com/helm/helm/issues/5153, https://github.com/helm/helm/issues/3503

Install topols Helm Repo.

```
helm repo add topols https://kvaster.github.io/topols
```

Install Helm Chart using the configured values.yaml.

```sh
helm upgrade --namespace=topols-system -f values.yaml -i topols topols/topols
```

## Configure kube-scheduler

`kube-scheduler` need to be configured to use `topols-scheduler` extender.

You need to copy `deploy/sheduler-config/scheduler-config.yaml` to `kube-scheduler`s config directory.
Usually it is located at `/etc/kubernetes` when you're deploying kubernetes with kubeadm.

```console
cp ./deploy/sheduler-config/scheduler-config.yaml /etc/kubernetes/
```

### For new clusters

If you are installing your cluster from scratch with `kubeadm`, you can use the following configuration:

```yaml
apiVersion: kubeadm.k8s.io/v1
kind: ClusterConfiguration
metadata:
  name: config
kubernetesVersion: v1.20.1
scheduler:
  extraVolumes:
    - name: "scheduler-config"
      hostPath: /etc/kubernetes/scheduler-config.yaml     # absolute path to ./scheduler-config.yaml file
      mountPath: /etc/kubernetes/scheduler-config.yaml
      pathType: FileOrCreate
      readOnly: true
  extraArgs:
    config: /etc/kubernetes/scheduler-config.yaml
```

### For existing clusters

First you should apply previous changes to ClusterConfiguration object in kubeadm's configmap:

```console
kubectl edit configmap -n kube-system kubeadm-config
```

Second you should apply theese changes in current configuration on each control plane node.
This may be done automatically using kubeadm or manually:

The changes to `/etc/kubernetes/manifests/kube-scheduler.yaml` that are affected by this are as follows:

1. Add a line to the `command` arguments array such as ```- --config=/etc/kubernetes/scheduler-config.yaml```.
   Note that this is the location of the file **after** it is mapped to the `kube-scheduler` container, not where it exists on the node local filesystem.
2. Add a volume mapping to the location of the configuration on your node:

    ```yaml
      spec.volumes:
      - hostPath:
          path: /etc/kubernetes/scheduler-config.yaml     # absolute path to ./scheduler-config.yaml file
          type: FileOrCreate
        name: scheduler-config
    ```

3. Add a `volumeMount` for the scheduler container:

    ```yaml
      spec.containers.volumeMounts:
      - mountPath: /etc/kubernetes/scheduler-config.yaml
        name: scheduler-config
        readOnly: true
    ```

## Configure available storage on each node

And final step is to configure available storage on each node.

1. Create directories or/and mounts in `/mnt/pool` for each device class (ssd, hdd e.t.c) you have.
   Each entry should be placed on a btrfs filesystem i.e.:
   * directory `/mnt/pool/hdd` - located on root filesystem which is btrfs
   * mount `/mnt/pool/ssd` - located on separate ssd drive with own btrfs filesystem
1. Create or modify config file `/mnt/pool/devices.yml`.

Config file is monitored and reapplied on each change.

Config file example:

```yaml
device-classes:
  - name: ssd
    default: true
    size: 100Gi
  - name: hdd
    size: 1Ti
```
