# Package Deploy Lifecycle

The following diagram shows the order of operations for the `zarf package deploy` command and the hook locations for [actions](../../examples/component-actions/README.md).

## `zarf package deploy`

```mermaid
graph TD
    B1(load package archive)-->B2
    B2(handle multipart package)-->B3
    B3(extract archive to temp dir)-->B4
    B4(filter components by architecture & OS)-->B5
    B5(save SBOM files to current dir)-->B6
    B6(handle deprecations)-->B9
    B9(confirm package deploy):::prompt-->B10
    B10(process deploy-time variables)-->B11
    B11(prompt for missing variables)-->B12
    B12(prompt to confirm components)-->B13
    B13(prompt to choose components in '.group')-->B14

    subgraph  
    B14(deploy each component)-->B14
    B14 --> B15(run each '.actions.onDeploy.before'):::action-->B15
    B15 --> B16(copy '.files')-->B17
    B17(load Zarf State)-->B18
    B18(push '.images')-->B19
    B19(push '.repos')-->B20
    B20(process '.dataInjections')-->B21
    B21(install '.charts')-->B22
    B22(apply '.manifests')-->B23
    B23(run each '.actions.onDeploy.after'):::action-->B23
    B23-->B24{Success?}
    B24-->|Yes|B25(run each\n'.actions.onDeploy.success'):::action-->B25
    B24-->|No|B26(run each\n'.actions.onDeploy.failure'):::action-->B26-->B999

    B999[Abort]:::fail
    end

    B25-->B27(print Zarf connect table)
    B27-->B28(save package data to cluster)


    classDef prompt fill:#4adede,color:#000000
    classDef action fill:#bd93f9,color:#000000
    classDef fail fill:#aa0000
```
