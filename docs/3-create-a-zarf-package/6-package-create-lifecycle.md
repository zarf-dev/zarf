# Package Create Lifecycle

The following diagram shows the order of operations for the `zarf package create` command and the hook locations for [actions](../../examples/component-actions/README.md).

## `zarf package create`

```mermaid
graph TD
    A1(set working directory)-->A2
    A2(parse zarf.yaml)-->A3
    A3(filter components by architecture)-->A4
    A4(detect init package)-->A5
    A5(handle deprecations)-->A6
    A6(parse component imports)-->A7
    A7(process create-time variables)-->A8
    A8(write build data and zarf.yaml)-->A9

    A9(run validations)-->A10
    A10(confirm package create):::prompt-->A11
    A11{Init package?}
    A11 -->|Yes| A12(add seed image)-->A13
    A11 -->|No| A13

    subgraph  
    A13(add each component)-->A13
    A13 --> A14(run each '.actions.onCreate.before'):::action-->A14
    A14 --> A15(load '.charts')-->A16
    A16(load '.files')-->A17
    A17(load '.dataInjections')-->A18
    A18(load '.manifests')-->A19
    A19(load '.repos')-->A20
    A20(run each '.actions.onCreate.after'):::action-->A20
    A20-->A21{Success?}
    A21-->|Yes|A22(run each\n'.actions.onCreate.success'):::action-->A22
    A21-->|No|A23(run each\n'.actions.onCreate.failure'):::action-->A23-->A999
    end

    A22-->A24(load all '.images')
    A24-->A25{Skip SBOM?}
    A25-->|Yes|A27
    A25-->|No|A26
    A26(generate SBOM)-->A27
    A27(reset working directory)-->A28
    A28(create package archive)-->A29
    A29{Is multipart?}
    A29-->|Yes|A30(split package archive)-->A31
    A29-->|No|A31
    A31(handle sbom view/out flags)

    A999[Abort]:::fail

    classDef prompt fill:#4adede,color:#000000
    classDef action fill:#bd93f9,color:#000000
    classDef fail fill:#aa0000
```
