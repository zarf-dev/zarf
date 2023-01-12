# Component Actions

:::note

Component Actions have replaced Component Scripts. Zarf will still read scripts entries, but will convert them to actions. Component Scripts will be removed in a future release.

:::

This example demonstrates how to define actions within your package that can run either on `zarf package create`, `zarf package deploy` or `zarf package remove`.  These actions will be executed with the context that the Zarf binary is executed with.

## Lifecycle of component actions

Provided a `zarf package <COMMAND>`, the following diagram shows the lifecycle of component actions:

```mermaid
graph TD
    A{zarf package} 
    A -->|create| B
    A -->|deploy| C
    A -->|remove| D

    B(each component 'actions.onCreate')-->B
    B -->E[load 'actions.onCreate.defaults']
    E -->F[each 'actions.onCreate.before']-->F
    F -->G{{run all zarf component steps}}
    G -->H[each 'actions.onCreate.after']-->H
    
    H-->I{Success?}
    I -->|Yes|J[each 'actions.onCreate.success']-->J
    I -->|No|K[each 'actions.onCreate.failure']-->K
    
    C(each component 'actions.onDeploy')-->C
    C -->M[load 'actions.onDeploy.defaults']
    M -->N[each 'actions.onDeploy.before']-->N
    N -->O{{run all zarf component steps}}
    O -->P[each 'actions.onDeploy.after']-->P

    P-->Q{Success?}
    Q -->|Yes|R[each 'actions.onDeploy.success']-->R
    Q -->|No|S[each 'actions.onDeploy.failure']-->S
    
    D(each component 'actions.onRemove')-->D
    D -->U[load 'actions.onRemove.defaults']
    U -->V[each 'actions.onRemove.before']-->V
    V -->W{{run all zarf component steps}}
    W -->X[each 'actions.onRemove.after']-->X

    X-->Y{Success?}
    Y -->|Yes|Z[each 'actions.onRemove.success']-->Z
    Y -->|No|AA[each 'actions.onRemove.failure']-->AA
```

## Prepare Scripts

`prepare` scripts run on `zarf package create` and allow a package creator to retrieve or manipulate files that they want to include in their Zarf package.  For example if you have a large data file that you need to include in your package you could include something like the following (replacing the url as needed):


```
components:
- name: prepare-example
  scripts:
    prepare:
    - wget https://download.kiwix.org/zim/wikipedia_en_100.zim
```

## Before Scripts

`before` scripts run on `zarf package deploy` and allow a package to execute commands _before_ the component is deployed into the cluster.  For example if you needed to create a infrastructure resources before a deployment:

```
components:
- name: before-example
  scripts:
    before:
    - "./eksctl create cluster -f eks.yaml"
```

## After Scripts

`after` scripts run on `zarf package deploy` and allow a package to execute commands _after_ the component is deployed into the cluster. For example if you need to cleanup resources that were temporarily created during deployment:

```
components:
- name: prepare-example
  scripts:
    after:
    - "rm my-temp-file.txt"
```

:::note

Any binaries you execute in your actions must exist on the machine they are executed on.

:::
