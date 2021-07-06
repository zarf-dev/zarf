The Zarf examples demonstrate different ways to utility Zarf in your environment.  All of these examples follow the same general release pattern and assume an offline / air-gapped deployment target.  

### [Appliance Mode](appliance/)
This example demonstrates using Zarf in a very low-resources/singlue-use environment.  In this mode there is no utility cluster and Zarf is simple a standard means of wrapping airgap concerns for K3s.  Appliance mode is also unique because you do not use anyting from the repo [releases](https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases) except the CLI.  This mode requires creating your own `zarf-initiazlize.tar.zst` to deploy the assets.  Though there are more complex patterns that could use the update process as well, for this example we only ever create the initial deployment.  Updates are done by re-creating the environment. 

