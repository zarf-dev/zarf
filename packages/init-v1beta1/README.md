### v1beta1 init package

This is an experimental v1beta1 package branch to evaluate the v1beta1 schema.

In order to utilize State values someone could run `zarf init --set-values=.injector.port=5002,.registry.mode=proxy`. This would be functionally equivalent to `zarf init --injector-port=5002 --registry-mode=proxy`.

There are also fields that are not available as flags since they are too niche, or don't play well as cli flags. The field `.injector.tolerations` is a good example of this and can be seen in example-deploy-values.yaml
