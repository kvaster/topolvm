[![Main](https://github.com/kvaster/topols/workflows/Main/badge.svg)](https://github.com/kvaster/topols/actions)

Introduction
============

TopoLS is fork of [TopoLVM](https://github.com/topolvm/topolvm) project.
TopoLS goal is the same as TopoLVM - provisioning of local storage, but TopoLS uses btrfs quotas instead of lvm2.
TopoLS runs in separate namespace and can be installed alongside with TopoLVM.

TopoLS code is ready for testing, but documentation is not yet finished (even touched), so please wait for corrected docs.

Last synced to TopoLVM code on 18 Mar 2024 at commit 44099e06250795cce5c6c18ac9b577fb04d01e29.

TopoLS
======

TopoLS is a [CSI][] plugin using btrfs quotas for Kubernetes.
It can be considered as a specific implementation of [local persistent volumes](https://kubernetes.io/docs/concepts/storage/volumes/#local) using CSI and LVM.

- **Project Status**: Testing for production
- **Conformed CSI version**: [1.6.0](https://github.com/container-storage-interface/spec/blob/v1.6.0/spec.md)

Supported environments
----------------------

- Kubernetes: 1.29, 1.28, 1.27
- Node OS: Linux
- Filesystems: btrfs

Features
--------

- [Dynamic provisioning](https://kubernetes-csi.github.io/docs/external-provisioner.html): Volumes are created dynamically when `PersistentVolumeClaim` objects are created.
- [Topology](https://kubernetes-csi.github.io/docs/topology.html): TopoLS uses CSI topology feature to schedule Pod to Node where free space on btrfs volume exists.
- Extended scheduler: TopoLS extends the general Pod scheduler to prioritize Nodes having larger storage capacity.
- Volume metrics: Usage stats are exported as Prometheus metrics from `kubelet`.
- [Volume Expansion](https://kubernetes-csi.github.io/docs/volume-expansion.html): Volumes can be expanded by editing `PersistentVolumeClaim` objects.
- [Storage capacity tracking](https://github.com/kvaster/topols/tree/main/deploy#storage-capacity-tracking): You can enable Storage Capacity Tracking mode instead of using topols-scheduler.

Programs
--------

This repository contains these programs:

- `topols-controller`: CSI controller service.
- `topols-scheduler`: A [scheduler extender](https://github.com/kubernetes/design-proposals-archive/blob/main/scheduling/scheduler_extender.md) for TopoLS.
- `topols-node`: CSI node service.

Getting started
---------------

For production deployments, see [deploy/README.md](./deploy/README.md).

Docker images
-------------

Docker images are available on `ghcr.io`:
[topols](https://github.com/users/kvaster/packages/container/package/topols),
[topols-with-sidecar](https://github.com/users/kvaster/packages/container/package/topols-with-sidecar).

[releases]: https://github.com/kvaster/topols/releases
[CSI]: https://github.com/container-storage-interface/spec
