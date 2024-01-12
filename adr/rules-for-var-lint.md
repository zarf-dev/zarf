The following are the rules we follow for zarf lint on files

- A variable can be declared by a user in three ways. Using --set, zarf config, or zarf variables
- Variables and constants work the same for these rules
- Builtin variables are not looked at
- A variable can be used in three ways. In files, a helm chart or manifests
  - If a variable is declared by a user and not used anywhere in the package then the user should get an error in lint:  unused variable
    - If a variable is declared by an imported component we will skip it if it is not used by the components we import as we don't know if it's actually being used or not.
  - If a variable is used by a package and not declared anywhere by a user then the user should get an error in lint: variable not declared
    - If a variable is declared by the imported package, the user should not recieve an error
