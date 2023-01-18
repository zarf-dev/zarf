# 10. Scripts -> Actions

Date: 2023-01-18

## Status

Accepted

## Context

Originally, the `scripts` noun was added to components to allow us to move hard-coded init business logic out of the codebase and into the package system. At the time there was only a `before` and `after` section with simply an array entry per command. Later, `prepare` was added as a way to do some thing during `zarf package create`. As teams began to find new ways to use the capabilities, their limitations became more obvious.

## Decision

The `scripts` section of the `zarf.yaml` will be replaced with a new `actions` section. The `actions` section will be a map of action names to a list of commands to run. `actions` will contain action sets that map to the following lifecycle events:

- `onCreate` - Runs during `zarf package create`
- `onDeploy` - Runs during `zarf package deploy`
- `onRemove` - Runs during `zarf package remove`

Each of these action sets are optional and contain different hooks and events that can be used to customize the package lifecycle:

- `defaults` - action configuration that will be applied to all actions in the set
- `before` - array of actions that will run before the component is processed for `create`, `deploy`, or `remove`
- `after` - array of actions that will run after the component processed for `create`, `deploy`, or `remove` successfully
- `onSuccess` - array of actions that will run after any `after` actions have successfully completed
- `onFailure` - array of actions that will run after any error during the above actions or component operations

Within aach `before`, `after`, `onSuccess`, and `onFailure` action set, the following configurations are available:

- `cmd` - (required) the command to run
- `dir` - the directory to run the command in
- `mute` - whether to mute the output of the command (default: `false`)
- `maxTotalSeconds` - the maximum total time to allow the command to run (default: `0` - no limit)
- `maxRetries` - the maximum number of times to retry the command if it fails (default: `0` - no retries)
- `env` - an array of environment variables to set for the command in the form of `name=value`
- `setVariable` - set the output of the command to a variable that can be used in other actions or components

Further details can be found in the `component-actions` [example package](../examples/component-actions/README.md) and the [component lifecycle documentation](../docs/4-user-guide/4-package-command-lifecycle.md).

## Consequences

With the current team agreement to not introduce breakng changing as we stabilize the API, a deprecation model was introduced that allows existing Zarf binaries to run with older `zarf.yaml` configs while also allowing the new features to be used by those who have updated their Zarf binary.
