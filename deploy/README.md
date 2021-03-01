Deploying TopoLS
================

Each of these steps are shown in depth in the following sections:

1. Prepare [cert-manager][] for topols-controller. You may supplement an existing instance.
1. Add `topols.kvaster.com/webhook: ignore` label to system namespaces such as `kube-system`.
1. Determine how topols-scheduler to be run:
   - If you run with a managed control plane (such as GKE, AKS, etc), `topols-scheduler` should be deployed as Deployment and Service
   - `topols-scheduler` should otherwise be deployed as DaemonSet in unmanaged (i.e. bare metal) deployments
1. Prepare StorageClasses for TopoLS.
1. Apply manifests for TopoLS from `deploy/manifests/base` plus overlays as appropriate to your installation.
1. Configure `kube-scheduler` to use `topols-scheduler`.
1. Configure available storage on each node.

Example configuration files are included in the following sub directories:

- `manifests/`: Manifests for Kubernetes.
- `scheduler-config/`: Configurations to extend `kube-scheduler` with `topolvm-scheduler`.
- `pool-config`: Configurations of available storage on each node. 

These configuration files may need to be modified for your environment.
Read carefully the following descriptions.

cert-manager
------------

[cert-manager][] is used to issue self-signed TLS certificate for topols-controller.
Follow the [documentation](https://docs.cert-manager.io/en/latest/getting-started/install/kubernetes.html) to install it into your Kubernetes cluster.

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

4. Edit `MutatingWebhookConfiguration` in [mutatingwebhooks.yaml](./manifests/base/mutatingwebhooks.yaml) as follows:

    ```yaml
    apiVersion: admissionregistration.k8s.io/v1beta1
    kind: MutatingWebhookConfiguration
    metadata:
      name: topols-hook
    # snip
    webhooks:
      - name: pvc-hook.topols.kvaster.com
        # snip
        clientConfig:
          caBundle: |  # Base64-encoded, PEM-encoded CA certificate that signs the server certificate
            ...
      - name: pod-hook.topols.kvaster.com
        # snip
        clientConfig:
          caBundle: |  # The same CA certificate as above
            ...
    ```
5. Remove `certificates.yaml` in [kustomization.yaml](./manifests/base/kustomization.yaml)

Protect system namespaces from TopoLS webhook
---------------------------------------------

TopoLS installs a mutating webhook for Pods.  It may prevent Kubernetes from bootstrapping
if the webhook pods and the system pods are both missing.

To workaround the problem, add a label to system namespaces such as `kube-system` as follows:

```console
$ kubectl label ns kube-system topols.kvaster.com/webhook=ignore
```

This label will be applied to the `topols-system` namespace via the `deploy/manifests/base/namespace.yaml` manifest.

topols-scheduler
-----------------

topols-scheduler is a [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for `kube-scheduler`.
It must be deployed to where `kube-scheduler` can connect.

If your Kubernetes cluster runs the control plane on Nodes, `topols-scheduler` should be run as DaemonSet
limited to the control plane nodes. `kube-scheduler` then connects to the extender via loopback network device.

Otherwise, `topols-scheduler` should be run as Deployment and Service.
`kube-scheduler` then connects to the Service address.

### Running topols-scheduler using DaemonSet

The [example manifest](./manifests/overlays/daemonset-scheduler/scheduler.yaml) can be used almost as is.
You may need to change the taint key or label name of the DaemonSet.

### Running topols-scheduler using Deployment and Service

In this case, you can use [deployment-scheduler/scheduler.yaml](./manifests/overlays/deployment-scheduler/scheduler.yaml) instead of [daemonset-scheduler/scheduler.yaml](./manifests/overlays/daemonset-scheduler/scheduler.yaml).

This way, `topols-scheduler` is exposed by LoadBalancer service.

Then edit `urlPrefix` in [scheduler-config.yaml](./scheduler-config/scheduler-config.yaml) to specify the LoadBalancer address.

OPTIONAL: tune the node scoring
-------------------------------

The node scoring for Pod scheduling can be fine-tuned with the following two ways:
1. Adjust `divisor` parameter in the scoring expression
2. Change the weight for the node scoring against the default by kube-scheduler

The scoring expression in `topols-scheduler` is as follows:
```
min(10, max(0, log2(capacity >> 30 / divisor)))
```
For example, the default of `divisor` is `1`, then if a node has the free disk capacity more than `1024GiB`, `topols-scheduler` scores the node as `10`. `divisor` should be adjusted to suit each environment. It can be specified the default value and values for each device-class in [scheduler-options.yaml](./manifests/overlays/daemonset-scheduler/scheduler-options.yaml) as follows:

```yaml
default-divisor: 1
divisors:
  ssd: 1
  hdd: 10
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

Prepare StorageClasses for TopoLS
---------------------------------

You need to create [StorageClasses](https://kubernetes.io/docs/concepts/storage/storage-classes/) for TopoLS.

An example is available in [provisioner.yaml](./manifests/base/provisioner.yaml) and it will be installed by default.

You may need/want to modify storage class name and also to enable provisioner by default:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topols    # ! EDIT NAME HERE !
  annotations:
    storageclass.kubernetes.io/is-default-class: true   # ! ADD THIS ANNOTATION FOR DEFAULT STORAGE CLASS !
provisioner: topols.kvaster.com
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

Apply manifests for TopoLS
--------------------------

Previous sections describe how to tune the manifest configurations, apply them now using Kustomize as follows:

### Running topols-scheduler using DaemonSet

If using `topols-scheduler` as a DaemonSet, run the following command: 

```console
kustomize build ./deploy/manifests/overlays/daemonset-scheduler | kubectl apply -f -
```

or use one built from default settings:

```console
kubectl apply -f deploy/topols.yaml`
```

### Running topols-scheduler using Deployment and Service

If using `topols-scheduler` as a Deployment, run the following command: 

```console
kustomize build ./deploy/manifests/overlays/deployment-scheduler | kubectl apply -f -
```

Configure kube-scheduler
------------------------

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

Configure available storage on each node
----------------------------------------

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

[cert-manager]: https://github.com/jetstack/cert-manager
