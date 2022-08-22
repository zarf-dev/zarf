import DocCardList from '@theme/DocCardList';
import {useCurrentSidebarCategory} from '@docusaurus/theme-common';

# Zarf Examples

The Zarf examples demonstrate different ways to utilize Zarf in your environment.  All of these examples follow the same general release pattern and assume an offline / air-gapped deployment target.

To build and deploy a demo, change directories to the example you want to try and run:

```shell
# This should be whatever example you want to try
cd <directory> # i.e. game

# This will create the zarf package
zarf package create

# This will prompt you to deploy the created zarf package
zarf package deploy
```

:::note

Examples are for demo purposes only and not meant for production use, they exist to demonstrate how to use Zarf in various ways. Modifying examples to fit production use is possible but will require additional configuration, time, and Kubernetes knowledge. Also, examples utilize software pulled from multiple sources and _some_ of them require authenticated access. Check the examples themselves for the specific accounts / logins required.

:::

&nbsp;

<DocCardList items={useCurrentSidebarCategory().items}/>
