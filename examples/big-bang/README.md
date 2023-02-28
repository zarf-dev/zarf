# Big Bang

This package deploys [Big Bang](https://repo1.dso.mil/platform-one/big-bang/bigbang) using the Zarf `bigbang` extension.  An example of this configuration is below:

```yaml
components:
  - name: bigbang
    required: true
    extensions:
      bigbang:
        version: 1.54.0
        skipFlux: false
        valuesFiles:
          - config/minimal.yaml #turns on just istio
          - config/ingress.yaml # adds istio certs for *.bigbang.dev
          - config/kyverno.yaml # turns on kyverno
          - config/loki.yaml # turns on loki and monitoring
```

The `bigbang` noun sits within the `extensions` specification of Zarf and provides the following configuration:

- `version`     - The version of Big Bang to use
- `repo`        - Override repo to pull Big Bang from instead of Repo One
- `skipFlux`    - Whether to skip deploying flux; Defaults to false
- `valuesFiles` - The list of values files to pass to Big Bang; these will be merged together

To walkthrough the creation and deployment of this package see the [Big Bang Walkthrough](../../docs/13-walkthroughs/5-big-bang.md).
