# Longhorn

This example shows how you how to deploy [Longhorn](https://longhorn.io/) using Zarf.

Before deploying Longhorn make sure your nodes are configured with the [Longhorn Installation Requirements](https://longhorn.io/docs/1.4.0/deploy/install/#installation-requirements).

You will need [open-iscsi](https://longhorn.io/docs/1.4.0/deploy/install/#installing-open-iscsi) installed.

If you wish to support RWX access modes you'll need to install an [NFSv4 client](https://longhorn.io/docs/1.4.0/deploy/install/#installing-nfsv4-client) on each node.

If you're working with K3s, there is extra setup required. See [Longhorn CSI on K3s](https://longhorn.io/docs/1.4.0/advanced-resources/os-distro-specific/csi-on-k3s/).

The values file from this example was pulled using the directions at [Customizing Default Settings](https://longhorn.io/docs/1.4.0/advanced-resources/deploy/customizing-default-settings/#using-helm) as the path for kubelet needs to be set for K3s as per [Longhorn CSI on K3s](https://longhorn.io/docs/1.4.0/advanced-resources/os-distro-specific/csi-on-k3s/)

You do not need to use the values file and can remove it from the Zarf package configuration if you're not using K3s and don't need that variable set.

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

``` bash
components:
  - name: longhorn
    required: true
    description: "Deploy Longhorn into a Kubernetes cluster.  https://longhorn.io"
    actions:
      # Run the Longhorn Environment Check on this cluster's nodes.
      onDeploy:
        before:
          - env: 
            - "PATH=$PATH:./"
          - cmd: ./environment_check.sh
      # Set the delete confirmation flag for Longhorn
      onRemove:
        before:
          - env:
            - "PATH=$PATH:./"
          - cmd: "kubectl -n longhorn-system patch -p '{\"value\": \"true\"}' --type=merge lhs deleting-confirmation-flag"
    files:
      # jq
      - source: https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64
        target: jq
        executable: true
      # kubectl
      - source: https://dl.k8s.io/release/v1.26.0/bin/linux/amd64/kubectl
        target: kubectl
        executable: true
      # Longhorn Environment Check
      - source: https://raw.githubusercontent.com/longhorn/longhorn/v1.4.0/scripts/environment_check.sh
        target: environment_check.sh
        executable: true
    manifests:
      - name: longhorn-connect
        namespace: longhorn-system
        files:
          - connect.yaml
    charts:
      - name: longhorn
        url:  https://charts.longhorn.io
        version: 1.4.0
        namespace: longhorn-system
        valuesFiles:
        - "values.yaml"
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
