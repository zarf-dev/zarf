This is a sample package that deploys Kafka onto K3s using Iron Bank images.

Steps to use:

1. Download the Zarf release, https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases
2. Run `zarf package create` in this folder on the online machine
3. Copy the created `zarf-package-kafka-strimzi-demo.tar.zst` file and the other download zarf files to the offline/airgap/test machine
4. Run `zarf init` with all defaults
5. Run `zarf package deploy` and choose the package from step 3.