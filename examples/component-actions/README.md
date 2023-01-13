# Component Actions

:::note

Component Actions have replaced Component Scripts. Zarf will still read scripts entries, but will convert them to actions. Component Scripts will be removed in a future release.

:::

This example demonstrates how to define actions within your package that can run either on `zarf package create`, `zarf package deploy` or `zarf package remove`. These actions will be executed with the context that the Zarf binary is executed with.

## Lifecycle of component actions

See [Zarf Package Lifecycle](../../docs/4-user-guide/4-package-command-lifecycle.md) for details on the execution sequence of component actions.

## Structure of component actions

Component actions align with the [Zarf Package Lifecycle](../../docs/4-user-guide/4-package-command-lifecycle.md) and are defined in the `zarf.yaml` file:

```yaml
components:
  - name: my-component
    actions:
      onCreate: # runs during zarf package create
        ...
      onDeploy: # runs during zarf package deploy
        ...
      onRemove: # runs during zarf package remove
        ...
```

Below the `actions.on*` key you can define the additional keys; the example below uses the `actions.onCreate` key:

```yaml
components:
  - name: my-component
    actions:
      onCreate:
        before: # runs before the component starts
          ...
        after: # runs after the component finishes
          ...
        onSuccess: # runs if the component finishes successfully
          ...
        onFailure: # runs if the component has an error
          ...
```

Below the `actions.on*.*`, an action can be defined as a list of commands to run, only `cmd` is required. The example below uses the `actions.onCreate.before` key:

```yaml
components:
  - name: my-component
    actions:
      onCreate:
        before:
          - cmd: "echo 'hello world'" # runs a command
            dir: tmp # runs the command in the tmp directory
            mute: true # hides the output of the command
            maxTotalSeconds: 10 # sets a timeout of 10 seconds for the command
            maxRetries: 3 # retries the command 3 times if it fails
            env:
              - demo=1 # sets the environment variable demo to 1
```

## `actions.onCreate`

`onCreate` runs during `zarf package create` and allow a package creator to run commands during package creation. For example if you have a large data file that you need to include in your package you could include something like the following (replacing the url as needed):

```yaml
components:
  - name: on-create-example
    actions:
      onCreate:
        before:
          - cmd: "wget https://download.kiwix.org/zim/wikipedia_en_100.zim"
```

## `actions.onDeploy`

`onDeploy` runs during `zarf package deploy` and allow a package to execute commands during component deployment.

You can use `onDeploy.before` to create execute a command _before_ the component is deployed. This example uses the `eksctl` binary to create an EKS cluster. The `eks.yaml` file is included in the package and contains the configuration for the cluster:

```yaml
components:
  - name: before-example
    actions:
      onDeploy:
        before:
          - cmd:"./eksctl create cluster -f eks.yaml"
```

You can also use `onDeploy.after` to execute a command _after_ the component is deployed. For example if you need to cleanup resources that were temporarily created during deployment:

```yaml
components:
  - name: prepare-example
    actions:
      onDeploy:
        after:
          - cmd: "rm my-temp-file.txt"
```

## `actions.onRemove`

`onRemove` runs during `zarf package remove` and allow a package to execute commands during component removal.

:::note

Any binaries you execute in your actions must exist on the machine they are executed on.

:::
