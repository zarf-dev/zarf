This example shows you how to deploy [Longhorn](https://longhorn.io/) using Zarf.

Before deploying Longhorn, make sure your nodes are configured with the [Longhorn Installation Requirements](https://longhorn.io/docs/latest/deploy/install/#installation-requirements). You can verify this automatically with Longhorn's own [preflight checker](https://github.com/longhorn/cli): install `longhornctl` and run `longhornctl check preflight --kubeconfig=<path-to-your-kubeconfig>` against the target cluster before deploying this package. It requires a real kubeconfig (it self-deploys its own short-lived DaemonSet across the cluster to run the checks), so it isn't included as a Zarf component here. Run it from your workstation as a pre-deploy step instead.

You will need [open-iscsi](https://longhorn.io/docs/latest/deploy/install/#installing-open-iscsi) installed.

If you wish to support RWX access modes you'll need to install an [NFSv4 client](https://longhorn.io/docs/latest/deploy/install/#installing-nfsv4-client) on each node.

If you're working with K3s, there is extra setup required. See [Longhorn CSI on K3s](https://longhorn.io/docs/latest/advanced-resources/os-distro-specific/csi-on-k3s/).

The values file from this example was pulled using the directions at [Customizing Default Settings](https://longhorn.io/docs/latest/advanced-resources/deploy/customizing-default-settings/#using-helm) as the path for kubelet needs to be set for K3s as per [Longhorn CSI on K3s](https://longhorn.io/docs/latest/advanced-resources/os-distro-specific/csi-on-k3s/)

You do not need to use the values file and can remove it from the Zarf package configuration if you're not using K3s and don't need that variable set.

The `global.imageRegistry` value in `values.yaml` is set to Zarf's `###ZARF_REGISTRY###` template variable. Zarf's mutating webhook already rewrites container image references on admission, so this value isn't needed for images to be pulled correctly. The webhook redirects any pod's image regardless of this setting. However, `longhorn-manager` is also passed its own expected image as a `--manager-image` command-line flag (rendered from this same `global.imageRegistry` value, independent of the webhook), which it compares against its own running pod's image to detect stale manager instances during upgrades. If `global.imageRegistry` is left unset, that flag never matches the webhook-rewritten pod image, and `longhorn-manager` gets stuck permanently treating its own fresh pod as stale. So this value is required for `longhorn-manager`'s own bootstrap logic, not just image resolution, and should not be removed.
