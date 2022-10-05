# 7. package variables files revisited

Date: 2022-08-24

## Status

Draft

## Context

This ADR is a follow up to the previous [package variables](./0006-package-variables.md) ADR to better refine variable use cases and consider impacts with the introduction of the ability to set variables from a file (Viper) in addition to interactive prompting and `--set`.  See the current flow below for additional context:

![Current Behavior](../.images/0007-package-variables-files-revisit/Current%20Behavior.drawio.svg)

The main issue that we seek to address from the previous implementation is that there is currently confusion around the use and resulting behavior of the `prompt` and `default` keys.

For the `default` / `prompt` cases, below is an example of the current behavior with the under the hood values annotated:

```yaml
variables:
  - name: "NOTHING"
    # default: ""   <- currently under the hood
    # prompt: false <- currently under the hood
  - name: "NO_PROMPT"
    default: "no-prompt"
    # prompt: false <- currently under the hood
  - name: "NO_DEFAULT"
    # default: ""   <- currently under the hood
    prompt: true
  - name: "BOTH"
    default: "both"
    prompt: true
```

```shell
zarf package deploy package.zst --confirm --set NOTHING=hello --set NO_PROMPT=world --set NO_DEFAULT=today --set BOTH=tommorow
# This currently works, setting NOTHING to "hello", NO_PROMPT to "world", NO_DEFAULT to "today" and BOTH to "tomorrow"

zarf package deploy package.zst --confirm --set NOTHING=hello --set NO_DEFAULT=today
# This currently works, setting NOTHING to "hello", NO_PROMPT to "no-prompt", NO_DEFAULT to "today" and BOTH to "both"

zarf package deploy package.zst --confirm --set NOTHING=hello
# This currently works, setting NOTHING to "hello", NO_PROMPT to "no-prompt", NO_DEFAULT to "" and BOTH to "both"

zarf package deploy package.zst --confirm
# This currently works, setting NOTHING to "", NO_PROMPT to "no-prompt", NO_DEFAULT to "" and BOTH to "both"

zarf package deploy package.zst
# This currently works, setting NOTHING to "" and NO_PROMPT to "no-prompt" and prompting the user for NO_DEFAULT and BOTH
```

This introduces confusion due to two separate issues/questions:

1. What if a package author doesn't want a `default` set?
1. What is the resulting behavior of `--confirm` and `prompt` in combination?

&nbsp;

## Options

With the above context below are options to address the concerns for both `prompt` and `default`, keeping in mind that file variables will be added.

&nbsp;

### Default


1. Remove the `default` key altogether and rely on package authors to create example overrides files:
    - *Pros*:
      - Overall simpler and allows us not to have to worry about abuses of the default key
    - *Cons*:
      - Removes options from package authors where they may want to set a legitimate default.
      - Overrides files don't natively travel along with the package
    - [Diagram](../.images/0007-package-variables-files-revisit/Remove%20Default%20Behavior.drawio.svg)

&nbsp;

2. Allow the `default` key to be unset if it does not exist (no implicit "")
    - *Pros*:
      - Allows package authors to set defaults that travel inside of a package
      - Defaults can potentially be carried into generated example override files
    - *Cons*:
      - Requires the use of a pointer in the package types definitions
    - [Diagram](../.images/0007-package-variables-files-revisit/Unsettable%20Default%20Behavior.drawio.svg)

&nbsp;

3. Introduce a `validation` key to match an input value to a regex
    - *Pros*:
      - Not a breaking change
      - Allows us to not have to have a pointer in our type definitions
    - *Cons*:
      - Does not remove the implicit default
      - Requires action of package authors
    - [Diagram](../.images/0007-package-variables-files-revisit/Validation%20Behavior.drawio.svg)

&nbsp;

4. Keep the current `default` behavior and force an implicit ""
    - *Pros*:
      - Not a breaking change
      - Allows us to not have to have a pointer in our type definitions
    - *Cons*:
      - Forces an implicit default onto package creators
    - [Diagram](../.images/0007-package-variables-files-revisit/Current%20Behavior.drawio.svg)

&nbsp;

### Prompt

1. Keep the current `--confirm` behavior but detect if there is a default set and error if both `prompt` and `--confirm` are used with no default or `--set` existing for a particular variable
    - *Pros*:
      - Overall simpler with less to type out at the command line
    - *Cons*:
      - Less explicit and could be confusing
      - Depends on implementing a `default` pointer
    - [Diagram](../.images/0007-package-variables-files-revisit/Unsettable%20Default%20Behavior.drawio.svg)

&nbsp;

2. Introduce a second flag (i.e. `--accept-defaults`) and error out if a user does not use the flag or set all variables through a file or at the command line
    - *Pros*:
      - More explicit
      - Does not rely on `default` COAs
    - *Cons*:
      - Requires another command line flag that may just get in the way
    - [Diagram](../.images/0007-package-variables-files-revisit/Accept%20Defaults%20Behavior.drawio.svg)

&nbsp;

3. Remove the `prompt` key and detect when to prompt based on `default` existing
    - *Pros*:
      - Makes things simpler overall
    - *Cons*:
      - Depends on implementing a `default` pointer
      - Gives package authors less control
    - [Diagram](../.images/0007-package-variables-files-revisit/Post-prompt%20Behavior.drawio.svg)

&nbsp;

4. Change the `prompt` key to `noPrompt` and default to prompting
    - *Pros*:
      - Makes the default more friendly to the interactive user by putting the prompt path in the path of least resistance
    - *Cons*:
      - Could make things unweildy in large packages by requiring the author to specify noPrompt
    - [Diagram](../.images/0007-package-variables-files-revisit/NoPrompt%20Behavior.drawio.svg)

&nbsp;

5. Dissallow `default: nil` and `prompt: false` to be used together on package create
    - *Pros*:
      - Moves potential errors from the package deployer to the package creator
      - Eliminates a potentially confusing case from existing
    - *Cons*:
      - Depends on implementing a `default` pointer
    - [Diagram](../.images/0007-package-variables-files-revisit/Error%20on%20Package%20Create%20Behavior.drawio.svg)

&nbsp;

## Decision

It was decided to allow the `default` key to be nillable ([Default #2](#default)), to change `prompt` to `noPrompt` ([Prompt #4](#prompt)), and to dissallow `noPrompt` and `default: nil` being combined at create time ([Prompt #5](#prompt)).  This would result in the following workflow:

![Decided Behavior](../.images/0007-package-variables-files-revisit/Decided%20Behavior.drawio.svg)

```yaml
variables:
  #  This case is now invalid and errors on package create
  #  - Package variables cannot use noPrompt: true without setting a default
  #
  #- name: "NOTHING"
  #  noPrompt: true
  #  default: nil  <- decided change
  - name: "NO_PROMPT"
    default: "no-prompt"
    noPrompt: true
  - name: "NO_DEFAULT"
    # noPrompt: false <- decided change
    # default: nil    <- decided change
  - name: "DEFAULT"
    default: "default"
    # noPrompt: false <- decided change
```

```shell
zarf package deploy package.zst --confirm --set NO_PROMPT=world --set NO_DEFAULT=today --set DEFAULT=tommorow
# This would continue to work, setting NO_PROMPT to "world", NO_DEFAULT to "today" and DEFAULT to "tomorrow"

zarf package deploy package.zst --confirm --set NO_DEFAULT=today
# This would continue to work, setting NO_PROMPT to "no-prompt", NO_DEFAULT to "today" and DEFAULT to "default"

zarf package deploy package.zst --confirm
# This would not work, erroring out with: "Variable NO_DEFAULT does not have a default, when using `--confirm` you must specify it with `--set` or `--set-file`."


zarf package deploy package.zst
# This would continue to work, setting NO_PROMPT to "no-prompt" and prompting for NO_DEFAULT and DEFAULT
```

## Consequences

This would require the following breaking changes:

- Any variables that do not currently have a `default` key specified have a new error state to handle on `--confirm`
- `noPrompt` (formerly `prompt: false`) must have a `default` key set in order to use that key
- Package authors must flip their `prompt` logic to `noPrompt`

This choice has the following benefits:

- Prompting is now the default which helps our interactive persona
- Errors asking the user to specify command flags only happen when they are already using the `--confirm` flag
- Implicit `""` defaults no longer are possible (authors can still explicitly set `""`)
- This choice does not exclude adding `validation` or `description` keys later on
