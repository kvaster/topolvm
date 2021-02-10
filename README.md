[![Main](https://github.com/kvaster/topols/workflows/Main/badge.svg)](https://github.com/kvaster/topols/actions)

Introduction
============

TopoLS is fork of [TopoLVM](https://github.com/topolvm/topolvm) project.
TopoLS goal is the same as TopoLVM - provisioning of local storage, but TopoLS uses btrfs quotas instead of lvm2.
TopoLS runs in separate namespace and can be installed alongside with TopoLVM.

TopoLS code is ready for testing, but documentation is not yet finished (even touched), so please wait for corrected docs.

Last synced to TopoLVM code on 10 feb 2021 at commit 844c8cccc45d0176778dc65a60cac791e145061c.

TopoLS
======

TopoLS is a [CSI][] plugin using btrfs quotas for Kubernetes.
It can be considered as a specific implementation of [local persistent volumes](https://kubernetes.io/docs/concepts/storage/volumes/#local) using CSI and LVM.

- **Project Status**: Testing for production
- **Conformed CSI version**: [1.3.0](https://github.com/container-storage-interface/spec/blob/v1.3.0/spec.md)

Supported environments
----------------------

- Kubernetes: 1.20, 1.19
- Node OS: Linux
- Filesystems: btrfs

Features
--------

- [Dynamic provisioning](https://kubernetes-csi.github.io/docs/external-provisioner.html): Volumes are created dynamically when `PersistentVolumeClaim` objects are created.
- [Ephemeral inline volume](https://kubernetes.io/docs/concepts/storage/volumes/#csi-ephemeral-volumes): Volumes can be directly embedded in the Pod specification.
- [Topology](https://kubernetes-csi.github.io/docs/topology.html): TopoLS uses CSI topology feature to schedule Pod to Node where free space on btrfs volume exists.
- Extended scheduler: TopoLS extends the general Pod scheduler to prioritize Nodes having larger storage capacity.
- Volume metrics: Usage stats are exported as Prometheus metrics from `kubelet`.
- [Volume Expansion](https://kubernetes-csi.github.io/docs/volume-expansion.html): Volumes can be expanded by editing `PersistentVolumeClaim` objects.

Programs
--------

This repository contains these programs:

- `topols-controller`: CSI controller service.
- `topols-scheduler`: A [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for TopoLS.
- `topols-node`: CSI node service.

Getting started
---------------

For production deployments, see [deploy](deploy/) directory.

Docker images
-------------

Docker images are available on [ghcr.io](https://github.com/users/kvaster/packages/container/package/topols)

[releases]: https://github.com/kvaster/topols/releases
[CSI]: https://github.com/container-storage-interface/spec
