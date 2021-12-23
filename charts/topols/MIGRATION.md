# Migration from Kustomize to Helm

## List of renamed resources

By becoming a Helm Chart, the Release Name is given to the `.metadata.name` of some resources.
As a result, there are some differences with the resources of TopoLS installed by Kustomize.

Old resources need to be deleted manually.

Helm template `{{template "topols.fullname" .}}` outputs Helm release name and chart name.
If release name contains chart name it will be used as a full name.

for example:

| Template | Release Name | Output |
| -------- |--------------| ------ |
| `{{ template "topols.fullname" . }}-controller` | foo          | **foo-topols-controller** |
| `{{ template "topols.fullname" . }}-controller` | topols       | **topols-controller** |
| `{{ template "topols.fullname" . }}-controller` | bar-topols   | **bar-topols-controller** |
| `{{ template "topols.fullname" . }}-controller` | topols-baz   | **topols-baz-controller** |

### List

| Kind | Kustomize Name                                                                                                                          | Helm Name |
| ---- |-----------------------------------------------------------------------------------------------------------------------------------------| --------- |
| Issuer             | [webhook-selfsign](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/certificates.yaml#L1-L9)                         | `{{ template "topols.fullname" . }}-webhook-selfsign` |
| Certificate        | [webhook-ca](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/certificates.yaml#L11-L27)                             | `{{ template "topols.fullname" . }}-webhook-ca` |
| Issuer             | [webhook-ca](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/certificates.yaml#L29-L37)                             | `{{ template "topols.fullname" . }}-webhook-ca` |
| Certificate        | [mutatingwebhook](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/certificates.yaml#L39-L58)                        | `{{ template "topols.fullname" . }}-mutatingwebhook` |
| CSIDriver          | [topols.kvaster.com](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L2-L11)                        | **NO CHANGED** |
| ServiceAccount     | [controller](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L14-L18)                               | `{{ template "topols.fullname" . }}-controller` |
| ClusterRole        | [topols-system:controller](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L20-L39)                 | `{{ .Release.Namespace }}:controller` |
| ClusterRoleBinding | [topols-system:controller](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L41-L52)                 | `{{ .Release.Namespace }}:controller` |
| Role               | [leader-election](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L54-L80)                          | **NO CHANGED** |
| RoleBinding        | [leader-election](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L82-L94)                          | **NO CHANGED** |
| ClusterRole        | [topols-external-provisioner-runner](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L96-L127)      | **NO CHANGED** |
| ClusterRoleBinding | [topols-csi-provisioner-role](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L129-L150)            | **NO CHANGED** |
| RoleBinding        | [csi-provisioner-role-cfg](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L152-L164)               | **NO CHANGED** |
| ClusterRole        | [topols-external-attacher-runner](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L166-L185)        | **NO CHANGED** |
| ClusterRoleBinding | [topols-csi-attacher-role](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L187-L198)               | **NO CHANGED** |
| RoleBinding        | [csi-attacher-role-cfg](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L210-L222)                  | **NO CHANGED** |
| ClusterRole        | [topols-external-resizer-runner](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L224-L240)         | **NO CHANGED** |
| ClusterRoleBinding | [topols-csi-resizer-role](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L242-L253)                | **NO CHANGED** |
| Role               | [external-resizer-cfg](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L255-L263)                   | **NO CHANGED** |
| RoleBinding        | [csi-resizer-role-cfg](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L265-L277)                   | **NO CHANGED** |
| Service            | [controller](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L280-L291)                             | `{{ template "topols.fullname" . }}-controller` |
| Deployment         | [controller](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/controller.yaml#L293-L399)                             | `{{ template "topols.fullname" . }}-controller` |
| MutatingWebhookConfiguration | [topols-hook](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/mutatingwebhooks.yaml#L1-L63)                         | `{{ template "topols.fullname" . }}-hook` |
| ServiceAccount     | [node](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/node.yaml#L1-L5)                                             | `{{ template "topols.fullname" . }}-node` |
| ClusterRole        | [topols-system:node](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/node.yaml#L7-L24)                              | `{{ .Release.Namespace }}:node` |
| ClusterRoleBinding | [topols-system:node](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/node.yaml#L26-L37)                             | `{{ .Release.Namespace }}:node` |
| DaemonSet          | [node](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/node.yaml#L40-L139)                                          | `{{ template "topols.fullname" . }}-node` |
| StorageClass       | [topols-provisioner](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/provisioner.yaml#L1-L9)                        | **NO CHANGED** |
| PodSecurityPolicy  | [topols-node](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/psp.yaml#L1-L27)                                      | `{{ template "topols.fullname" . }}-node` |
| PodSecurityPolicy  | [topols-scheduler](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/psp.yaml#L29-L55)                                | `{{ template "topols.fullname" . }}-scheduler` |
| ServiceAccount     | [topols-system](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/scheduler.yaml#L2-L6)                               | `{{ template "topols.fullname" . }}-scheduler` |
| Role               | [psp:topols-scheduler](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/scheduler.yaml#L8-L17)                       | `psp:{{ template "topols.fullname" . }}-scheduler` |
| RoleBinding        | [topols-scheduler:psp:topols-scheduler](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/base/scheduler.yaml#L19-L31)     | `{{ template "topols.fullname" . }}-scheduler:psp:{{ template "topols.fullname" . }}-scheduler` |
| DaemonSet          | [topols-scheduler](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/overlays/daemonset-scheduler/scheduler.yaml#L1-L54)   | `{{ template "topols.fullname" . }}-scheduler` |
| Deployment         | [topols-scheduler](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/overlays/deployment-scheduler/scheduler.yaml#L1-L36)  | `{{ template "topols.fullname" . }}-scheduler` |
| Service            | [topols-scheduler](https://github.com/kvaster/topols/blob/v0.8.3/deploy/manifests/overlays/deployment-scheduler/scheduler.yaml#L38-L49) | `{{ template "topols.fullname" . }}-scheduler` |
