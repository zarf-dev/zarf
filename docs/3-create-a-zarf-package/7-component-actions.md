# Component Actions

Component Actions offer several exec entrypoints that allow a component to perform additional logic at key stages of its lifecycle. These actions are executed within the same context as the Zarf binary. For a detailed overview of the execution sequence of component actions, please refer to the Zarf [package create lifecycle documentation](./5-package-create-lifecycle.md), [package deploy lifecycle documentation](../4-deploy-a-zarf-package/1-package-deploy-lifecycle.md). Additionally, you can experiment with the component actions example located in the [Component Actions](../../examples/component-actions/README.md) example page.

## Action Sets

The `component.actions` field includes the following optional keys, also known as `action sets`:

- `onCreate` - Runs during `zarf package create`.
- `onDeploy` - Runs during `zarf package deploy`.
- `onRemove` - Runs during `zarf package remove`.

## Action Lists

These `action sets` contain optional `action` lists. The `onSuccess` and `onFailure` action lists are conditional and rely on the success or failure of previous actions within the same component, as well as the component's lifecycle stages.

- `before` - sequential list of actions that will run before this component is processed for `create`, `deploy`, or `remove`.
- `after` - sequential list of actions that will run after this component is successfully processed for `create`, `deploy`, or `remove`.
- `onSuccess` - sequential list of actions that will run after **ALL** `after` actions have successfully completed.
- `onFailure` - sequential list of actions that will run after **ANY** error during the above actions or component operations.

Below are some examples of `action` lists and their usages:

```yaml
components:
  - name: on-create
    actions:
      # runs during "zarf package create"
      onCreate:
        # runs before the component is created
        before:
          # on Windows, touch is replaced with New-Item
          - cmd: touch test-create-before.txt
            # dir is the directory to run the command in
            dir: ''
            # this environment variable will be set for this action only
            env:
              - 'thing=stuff'
            # the number of times to retry the action if it fails
            maxRetries: 0
            # the maximum amount of times the action can run before it is killed, including retries
            maxTotalSeconds: 30
            # determine if actions output should be printed to the console
            mute: false
        # runs after the component is deployed
        after:
          - cmd: touch test-create-after.txt

  - name: on-deploy-with-dynamic-variable
    actions:
      # runs during "zarf package deploy"
      onDeploy:
        # runs before the component is deployed
        before:
          # setVariables can be used to set variables for use in other actions or components (only onDeploy)
          - cmd: echo "meow"
            setVariables:
              - name: CAT_SOUND
                # these variables can also (optionally) be marked as sensitive to sanitize them in the Zarf log
                sensitive: true
          # NOTE: when including a variable in a command output this will be written to the log regardless of the sensitive setting
          # - use `mute` to silence the command output for sensitive variables
          - cmd: echo "the cat says ${ZARF_VAR_CAT_SOUND}"
            mute: true

  - name: on-deploy-with-timeout
    description: This component will fail after 1 second
    actions:
      # runs during "zarf package deploy"
      onDeploy:
        # defaults allow you to specify default values for the actions in that acitonSet
        defaults:
          # maxTotalSeconds is the maximum amount of time the action can run before it is killed, including retries
          maxTotalSeconds: 1
          # maxRetries is the maximum number of times the action will be retried on failure
          maxRetries: 3
        before:
          # this action will fail after 1 second
          - cmd: sleep 30
        onFailure:
          - cmd: echo "😭😭😭 this action failed because it took too long to run 😭😭😭"
```

## Action Set Defaults

In addition to the `action` lists above, an `action set` also contains a `defaults` section that will be applied to all actions in the set. The `defaults` section contains all of the same elements as an action configuration, with the exception of the `cmd` element, which is not allowed in the `defaults` section. Below is an example of `action set` defaults:

```yaml
actions:
  onCreate:
    defaults:
      # Set the default directory for all actions in this action set (onCreate)
      dir: dir-1
    before:
      # dir-1 will be used for these action
      - cmd: echo "before"
      # dir-2 will be used for this action
      - cmd: echo "before"
        dir: dir-2
    after:
      # dir-1 will be used for these actions
      - cmd: echo "after"
  onDeploy:
    before:
      # this action will use the current working directory
      - cmd: echo "before"
```

## Common Action Configuration Keys

The following keys are common to all action configurations (wait or command):

- `description` - a description of the action that will replace the default text displayed to the user when the action is running. For example: `description: "File to be created"` would display `Waiting for "File to be created"` instead of `Waiting for "touch test-create-before.txt"`.
- `maxTotalSeconds` - the maximum total time to allow the command to run (default: `0` - no limit for command actions, `300` - 5 minutes for wait actions).

## Command Action Configuration

A command action executes arbitrary commands or scripts within a shell wrapper. You can use the `cmd` key to define the command(s) to run. This can also be a multi-line script. _You cannot use `cmd` and `wait` in the same action_.

Within each of the `action` lists (`before`, `after`, `onSuccess`, and `onFailure`), the following action configurations are available:

- `cmd` - (required if not a wait action) the command to run.
- `dir` - the directory to run the command in, defaults to the current working directory.
- `mute` - whether to mute the realtime output of the command, output is always shown at the end on failure (default: `false`).
- `maxRetries` - the maximum number of times to retry the command if it fails (default: `0` - no retries).
- `env` - an array of environment variables to set for the command in the form of `name=value`.
- `setVariables` - set the standard output of the command to a list of variables that can be used in other actions or components (onDeploy only).

## Wait Action Configuration

The `wait` action temporarily halts the component stage it's initiated in, either until the specified condition is satisfied or until the maxTotalSeconds time limit is exceeded (which, by default, is set to 5 minutes). To define `wait` parameters, execute the `wait` key; it's essential to note that _you cannot use `cmd` and `wait` in the same action_. Essentially, a `wait` action is _yaml sugar_ for a call to `./zarf tools wait-for`.

Within each of the `action` lists (`before`, `after`, `onSuccess`, and `onFailure`), the following action configurations are available:

- `wait` - (required if not a cmd action) the wait parameters.
  - `cluster` - perform a wait operation on a Kubernetes resource (kubectl wait).
    - `kind` - the kind of resource to wait for (required).
    - `name` - the name of the resource to wait for (required), can be a name or label selector.
    - `namespace` - the namespace of the resource to wait for.
    - `condition` - the condition to wait for (default: `exists`).
  - `network` - perform a wait operation on a network resource (curl).
    - `protocol` - the protocol to use (i.e. `http`, `https`, `tcp`).
    - `address` - the address/port to wait for (required).
    - `code` - the HTTP status code to wait for if using `http` or `https`, or `success` to check for any 2xx response code (default: `success`).

---

## Creating Dynamic Variables from Actions

You can use the `setVariables` action configuration to set a list of variables that can be used in other actions or components during `zarf package deploy`. The variable will be assigned values in two environment variables: `ZARF_VAR_{NAME}` and `TF_VAR_{name}`. These values will be accessible in subsequent actions and can be used for templating in files or manifests in other components as `###ZARF_VAR_{NAME}###`. This feature allows package authors to define dynamic runtime variables for consumption by other components or actions. _Unlike normal variables, these do not need to be defined at the top of the `zarf.yaml`._

## Additional Action Examples

### `onCreate`

The `onCreate` action runs during `zarf package create` and allows a package creator to run commands during package creation. For instance, if a large data file must be included in your package, the following example (with the URL updated accordingly) can be used:

```yaml
components:
  - name: on-create-example
    actions:
      onCreate:
        before:
          - cmd: 'wget https://download.kiwix.org/zim/wikipedia_en_100.zim'
```

### `onDeploy.before`

The `onDeploy` action runs during `zarf package deploy` and allow a package to execute commands during component deployment.

You can use `onDeploy.before` to execute a command _before_ the component is deployed. The following example uses the `eksctl` binary to create an EKS cluster. The package includes the `eks.yaml` file, which contains the cluster configuration:  

```yaml
components:
  - name: before-example
    actions:
      onDeploy:
        before:
          - cmd:"./eksctl create cluster -f eks.yaml"
```

### `onDeploy.after`

The `onDeploy.after` can be used to execute a command _after_ the component is deployed. This can be useful for resource cleanup of any temporary resources created during the deployment process:

```yaml
components:
  - name: prepare-example
    actions:
      onDeploy:
        after:
          - cmd: 'rm my-temp-file.txt'
```

### `onRemove`

The `onRemove` action runs during `zarf package remove` and allows a package to execute commands during component removal.

:::note

Any binaries you execute in your actions must exist on the machine they are executed on.

:::
