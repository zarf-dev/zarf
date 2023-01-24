# Component Actions

:::note

Component Actions have replaced Component Scripts. Zarf will still read scripts entries, but will convert them to actions. Component Scripts will be removed in a future release.

:::

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

This example demonstrates how to define actions within your package that can run either on `zarf package create`, `zarf package deploy` or `zarf package remove`. These actions will be executed with the context that the Zarf binary is executed with.

For more details on component actions, see the [component actions](../../docs/4-user-guide/5-component-actions.md) documentation.

```yaml
components:
  - name: on-create
    actions:
      # runs during "zarf package create"
      onCreate:
        # defaults are applied to all actions in this actionSet
        defaults:
          dir: ''
          env: []
          maxRetries: 0
          maxTotalSeconds: 30
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
            # the name of the variable to set with the output of the action
            setVariable: BLEH
        # runs after the component is deployed
        after:
          - cmd: touch test-create-after.txt

- name: on-deploy-with-dynamic-variable
    actions:
    # runs during "zarf package deploy"
    onDeploy:
        # runs before the component is deployed
        before:
        # setVariable can be used to set a variable for use in other actions or components
        - cmd: echo "meow"
            setVariable: CAT_SOUND
        # this action will have access to the variable set in the previous action
        - cmd: echo "the cat says ${ZARF_VAR_CAT_SOUND}"

```
