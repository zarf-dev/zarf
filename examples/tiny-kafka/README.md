This is a sample package that deploys Kafka onto K3s using Iron Bank images.

Steps to use:

1. Download the Zarf release, https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases
2. Run `zarf package create` in this folder on the online machine
3. Copy the created `zarf-package-kafka-strimzi-demo.tar.zst` file and the other download zarf files to the offline/airgap/test machine
4. Run `zarf init` with all defaults
5. Run `zarf package deploy` and choose the package from step 3.

Testing will require JDK and the kafka tools:  `sudo apt install openjdk-14-jdk-headless` for Ubuntu.  More details can be found at https://kafka.apache.org/quickstart.  Steps to test:

1. Install JDK and extract the Kafka tools from the package `/opt/kafka.tgz`
2. Get the Nodeport: `NODEPORT=$(kubectl get service demo-kafka-external-bootstrap -n kafka-demo -o=jsonpath='{.spec.ports[0].nodePort}{"\n"}')`
3. For pub: `./bin/kafka-console-producer.sh --broker-list localhost:$NODEPORT --topic cool-topic`
4. For sub: `./bin/kafka-console-consumer.sh --bootstrap-server localhost:$NODEPORT --topic cool-topic`
