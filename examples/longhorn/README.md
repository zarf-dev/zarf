# Longhorn

This example shows you how to deploy [Longhorn](https://longhorn.io/) using Zarf.

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
  - name: longhorn-environment-check
    required: true
    files:
      - source: https://raw.githubusercontent.com/longhorn/longhorn/v1.4.0/scripts/environment_check.sh
        target: environment_check.sh
        executable: true
      - source: https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64
        target: jq
        executable: true
    actions:
      # Run the Longhorn Environment Check on this cluster's nodes.
      onDeploy:
        after:
          - cmd: |
              export PATH=$PATH:./
              awk '{gsub(/kubectl /, "./zarf tools kubectl ")} 1' ./environment_check.sh > tmp && mv tmp ./environment_check.sh
              awk '{gsub(/"kubectl" /, "")} 1' ./environment_check.sh > tmp && mv tmp ./environment_check.sh
              chmod +x ./environment_check.sh
              ./environment_check.sh
  - name: longhorn
    required: true
    description: "Deploy Longhorn into a Kubernetes cluster.  https://longhorn.io"
    actions:
      # Set the delete confirmation flag for Longhorn
      onRemove:
        before:
          - cmd: "./zarf tools kubectl -n longhorn-system patch -p '{\"value\": \"true\"}' --type=merge lhs deleting-confirmation-flag"
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
