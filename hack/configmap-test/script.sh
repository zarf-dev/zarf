# this happens for sure on v1.32.10
# effected versions:
# v1.34.2+k3s1
# v1.33.6+k3s
# This does not happen on kind built with latest kubernetes, leading me to believe it is a k3s issue.
loop_count=0
while true; do
  loop_count=$((loop_count + 1))
  echo "Loop iteration: $loop_count"
  k3d cluster delete
  k3d cluster create --image rancher/k3s:v1.32.10-k3s1  || continue
  kubectl create ns zarf
  go run main.go || break
done
