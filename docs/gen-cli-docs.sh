# Generate the Zarf CLI docs
printf "Generating CLI docs\n"
ZARF_CONFIG=hack/empty-config.toml go run main.go internal generate-cli-docs

# Create the top menu label for the sidebar menu
printf "Generating sidebar menu label\n"
MENU_LABEL='{"label": "CLI Commands"}\n'
printf "${MENU_LABEL}" > docs/2-the-zarf-cli/100-cli-commands/_category_.json

# The GenMarkdownTree function from cobra/docs starts the headers at H2.
# This breaks the sidebar menu naming for Docusaurus. This command drops
# all headers by 1 level to fix the menu.
printf "Updating section header levels\n"
for FILE in `find docs/2-the-zarf-cli/100-cli-commands -name "*.md"`
do
  sed -i.bak 's/^##/#/g' ${FILE}
  sed -i.bak '2s/^/<!-- Auto-generated by docs\/gen-cli-docs.sh -->\n/' ${FILE}
  truncate -s -1 ${FILE}
  rm ${FILE}.bak
done
