Zarf includes an unmodified version of the Busybox Linux.  The sourcecode used to build the busybox binaries in this directory can be found in the `busybox-source.tgz` file in this same directory.  Additionally, the required config filees to produce these binaries are also found in this directory with their cooresponding architectures.  These binaries were built natively on their respective platforms without the use of cross-compilers.  Please refer to the complete license text in the LICENSE file in this same folder.


Steps to test (amd64):

`kubectl create ns zarf`

If replacing injection

`kubectl -n zarf delete configmap injector-binaries`

Add the binaries as a configmap

`kubectl create configmap -n zarf injector-binaries --from-file=busybox=busybox-amd64 --from-file=init.sh`

`kubectl apply -f inject.yaml`

Once it's running

`zarf connect seed-registry --local-port 5000`

Send the files

`./busybox-amd64 tar cv zarf seed-image.tar | ./busybox-amd64 netcat 127.0.0.1 5000`