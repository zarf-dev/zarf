#!/usr/bin/env sh

echo "Checking zarf.schema.json..."
cmp -s zarf.schema.json zarf.schema.json.bak

echo "Checking 3-zarf-schema.md..."
cmp -s docs/4-user-guide/3-zarf-schema.md docs/4-user-guide/3-zarf-schema.md.bak

echo "Checking CLI Docs..."
DIFF=$(for f in `find docs/4-user-guide/1-the-zarf-cli/100-cli-commands/* ! -type l`;do diff -rq $f docs/4-user-guide/1-the-zarf-cli/100-cli-commands.bak/${f##*/};done)

if [ -z "$DIFF" ]; then
    echo "Success!"
    exit 0
else
    echo $DIFF
    exit 1
fi
