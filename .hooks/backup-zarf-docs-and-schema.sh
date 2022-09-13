#!/usr/bin/env sh

cp zarf.schema.json zarf.schema.json.bak
cp docs/4-user-guide/3-zarf-schema.md docs/4-user-guide/3-zarf-schema.md.bak
mkdir docs/4-user-guide/1-the-zarf-cli/100-cli-commands.bak/
cp -r docs/4-user-guide/1-the-zarf-cli/100-cli-commands/* docs/4-user-guide/1-the-zarf-cli/100-cli-commands.bak/
