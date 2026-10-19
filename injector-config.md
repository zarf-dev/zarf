## Injector Image

Registry proxy mode requires an explicit image through the `--injector-image` flag. This image must be available on every node before the cluster is initialized. The pause image is a good choice as it is small, has no shell, and must already exist on a node to start any pod. To find the pause image run:

```bash
zarf tools kubectl get node <node-name> -o json | jq -r '.status.images[].names[] | select(contains("pause"))'
```

Note: Node's default to storing the last 50 images pulled so this command may fail to find an image. Usually, you can find the default pause image used by distributions in their documentation. Otherwise, any image will work, so long as every Node either has it pre-staged in the image store, or has the ability to pull it.
