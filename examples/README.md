The Zarf examples demonstrate different ways to utility Zarf in your environment.  All of these examples follow the same general release pattern and assume an offline / air-gapped deployment target.  

### [Appliance Mode](appliance/)
This example demonstrates using Zarf in a very low-resources/singlue-use environment.  In this mode there is no utility cluster and Zarf is simply a standard means of wrapping airgap concerns for K3s.  Using appliance mode requires the `zarf-appliance-init.tar.zst` file from the [releases](https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases) and the command line flag `--appliance-mode` when running `zarf init`.  

