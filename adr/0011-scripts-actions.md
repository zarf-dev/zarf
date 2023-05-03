# 11. Scripts -> Actions

Date: 2023-01-18

## Status

Accepted

## Context

Originally, the `scripts` noun was added to components to allow us to move hard-coded init business logic out of the codebase and into the package system. At the time there was only a `before` and `after` section with simply an array entry per command. Later, `prepare` was added as a way to do something during `zarf package create`. As teams began to find new ways to use the capabilities, their limitations became more obvious.

## Decision

The `scripts` section of the `zarf.yaml` will be replaced with a new `actions` section. The `actions` section will be a map of action names to a list of commands to run. `actions` will contain `action sets` that map to the following lifecycle events:

- `onCreate` - Runs during `zarf package create`
- `onDeploy` - Runs during `zarf package deploy`
- `onRemove` - Runs during `zarf package remove`

In addition to adding more lifecycle events, the `actions` section will also allow for more complex actions to be defined. New configurations include, setting the cmd directory, defining custom env variables, setting the number of retries, setting the max total seconds, muting the output, and [setting a variable](../docs/3-create-a-zarf-package/7-component-actions.md#creating-dynamic-variables-from-actions) to be used in other actions or components.

Further details can be found in the `component-actions` [component actions documentation](../docs/3-create-a-zarf-package/7-component-actions.md), [package create lifecycle documentation](../docs/3-create-a-zarf-package/6-package-create-lifecycle.md), [package deploy lifecycle documentation](../docs/4-deploy-a-zarf-package/2-package-deploy-lifecycle.md), and the [example package](../examples/component-actions/README.md).

## Consequences

With the current team agreement to not introduce breaking changes as we stabilize the API, a deprecation model was introduced that allows existing Zarf binaries to run with older `zarf.yaml` configs while also allowing the new features to be used by those who have updated their Zarf binary.
