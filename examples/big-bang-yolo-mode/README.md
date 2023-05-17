# Big Bang (YOLO Mode)

This package deploys [Big Bang](https://repo1.dso.mil/platform-one/big-bang/bigbang) using the Zarf `bigbang` extension with YOLO mode enabled. You can learn about YOLO mode [here](https://docs.zarf.dev/docs/faq#what-is-yolo-mode-and-why-would-i-use-it).  An example of this configuration is below:

```yaml
components:
  - name: flux-private-registry
    required: true
    manifests:
      - name: private-registry
        namespace: flux-system
        files:
          - secrets/private-registry.yaml
  - name: bigbang
    required: true
    extensions:
      bigbang:
        version: 2.0.0
        valuesFiles:
          - config/credentials.yaml
          - config/ingress.yaml
          - config/kyverno.yaml
          - config/loki.yaml
```

The `provision-flux-credentials` component is required to create the necessary secret to pull flux images from [registry1.dso.mil](https://registry1.dso.mil). In the provided `zarf.yaml` for this example, we demonstrate providing account credentials via Zarf Variables, although there are other ways to populate the data in `private-registry.yaml`.
