# Longhorn

This example shows how you how to deploy [Longhorn](longhorn.io) using Zarf

Before deploying Longhorn make sure your nodes are configured with the [Longhorn Installation Requirements](https://longhorn.io/docs/1.4.0/deploy/install/#installation-requirements).

You will need [open-iscsi](https://longhorn.io/docs/1.4.0/deploy/install/#installing-open-iscsi) installed.

If you wish to support RWX access modes you'll need to install an [NFSv4 client](https://longhorn.io/docs/1.4.0/deploy/install/#installing-nfsv4-client) on each node.

If you're working with K3s, there is extra setup required see [Longhorn CSI on K3s](https://longhorn.io/docs/1.4.0/advanced-resources/os-distro-specific/csi-on-k3s/).

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

```
components:
  - name: longhorn
    required: true
    description: "Deploy Longhorn into a Kubernetes cluster.  https://longhorn.io"
    charts:
      - name: longhorn
        url:  https://charts.longhorn.io
        version: 1.4.0
        namespace: longhorn-system
    images:
      - longhornio/csi-attacher:v3.4.0
      - longhornio/csi-provisioner:v2.1.2
      - longhornio/csi-resizer:v1.3.0
      - longhornio/csi-snapshotter:v5.0.1
      - longhornio/csi-node-driver-registrar:v2.5.0
      - longhornio/livenessprobe:v2.8.0
      - longhornio/backing-image-manager:v1.4.0
      - longhornio/longhorn-engine:v1.4.0
      - longhornio/longhorn-instance-manager:v1.4.0
      - longhornio/longhorn-manager:v1.4.0
      - longhornio/longhorn-share-manager:v1.4.0
      - longhornio/longhorn-ui:v1.4.0
      - longhornio/support-bundle-kit:v0.0.17
```
