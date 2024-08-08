# SOCI on Kubernetes

This document explains how to configure SOCI on Kubernetes. For a hands on example, see [SOCI on Amazon Elastic Kubernetes Service (EKS)](./eks.md).

> **Note**
>
> SOCI on Kubernetes has a few rough edges that you should consider before
> using it for important workloads. See [Limitations](#limitations).  
> We welcome your feedback and suggestions for improving the Kubernetes experience.

## Configuration

SOCI on kubernetes requires two pieces of configuration:
1) [Containerd Configuration](#containerd-configuration) to launch containers with SOCI
2) [Registry Authentication Configuration](#registry-authentication-configuration) so that SOCI can pull images from non-public container registries

### Containerd configuration

To configure containerd to launch containers with SOCI, add the following snippet to the containerd config. The config is located at `/etc/containerd/config.toml` by default. 

```
[proxy_plugins.soci]
type = "snapshot"
address = "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"
[proxy_plugins.soci.exports]
  root = "/var/lib/soci-snapshotter-grpc"

[plugins."io.containerd.grpc.v1.cri".containerd]
  snapshotter = "soci"
  # This line is required for containerd to send information about how to lazily load the image to the snapshotter
  disable_snapshot_annotations = false
```

> **NOTE**
>
> Your config might already have the 
> `[plugins."io.containerd.grpc.v1.cri".containerd]` section in which case you should add the `snapshotter` and `disable_snapshot_annotations` lines to the existing section rather than defining a new one.

Breaking it down line-by-line:  
`[proxy_plugins.soci]` makes containerd aware of the SOCI plugin  
`type = "snapshot"` tells containerd that the SOCI plugin is a snapshotter and implements the snapshotter API  
`address = "/run/soci-snapshotter-grpc/soci-snapshotter-grpc.sock"` tells containerd where to connect to the SOCI snapshotter 
`[proxy_plugins.soci.exports]` defines a set of metadata  
`    root = "/var/lib/soci-snapshotter-grpc"` defines the root data directory for the SOCI snapshotter. Kubernetes uses this to calculate disk utilization, enforce storage limits, and trigger garbage collection.

`[plugins."io.containerd.grpc.v1.cri".containerd]` defines kubernetes-specific configuration  
`  snapshotter = "soci"` tells containerd to use SOCI by default. This name must match the proxy_plugin name. (this is required. See [Limitations](#limitations))  
`  disable_snapshot_annotations = false` tells containerd to send lazy loading information to the SOCI snapshotter  

### Registry Authentication Configuration

The SOCI snapshotter lazily pulls image content outside of the normal image pull context. As a result, it must be independently configured to receive credentials to access non-public container registries.

There are several mechanisms to configure SOCI to access non-public container registries with different trade-offs. See the [registry authentication documentation](./registry-authentication.md) for full information. Choosing a specific mechanism requires evaluating which set of trade-offs best suits a particular use-case.

If you are looking to quickly evaluate SOCI on Kubernetes or you are unsure what the trade-offs look like in practice, [Kubernetes CRI Credentials](./registry-authentication.md#kubernetes-cri-credentials) will work with widest range of use-cases. You should verify that the trade-offs are appropriate for your use-case before using this for important workloads.

## Limitations

1. **SOCI must be used for all containers on the node**
    Kubernetes has its own view of images that is not containerd snapshotter-aware. For example, If an image has been pulled with the default OverlayFS and then a pod is scheduled with SOCI, Kubernetes will not pull the image with the new snapshotter. The pod launch will fail because the image is not found in the SOCI snapshotter.
    
1. **SOCI must be configured at node launch**
    Related to the previous limitation, if SOCI is not configured at launch time, then the pause container will be pulled with the default OverlayFS snapshotter. As a result, SOCI pods will not launch because the pause container is not found in the SOCI snapshotter.

## Known Bugs

1. **containerd <1.7.16 does not enforce storage limits or run garbage collection**  
    A bug in containerd caused the Kubelet to calculate SOCI snapshotter disk utilization incorrectly which broke kubernetes garbage collection and prevented enforcement of storage limits.


