## Injector Image

Registry proxy mode requires an explicit image reference through the `--injector-image` flag when initializing Zarf for the first time. Subsequent initializations can reuse the image saved in Zarf state. Because the injector is deployed as a DaemonSet, every node must have the specified image cached locally or be able to pull it independently of Zarf's registry.

The pause image is a good choice because it is already available on nodes running pods. To find it, run:

```bash
zarf tools kubectl get node <node-name> -o yaml | zarf tools yq -r '.status.images[].names[] | select(contains("pause"))'
```

By default, kubelet reports only the 50 largest images in node status, so this command may return nothing even when the pause image is present. If necessary, check your distribution's documentation to find its default pause image.
