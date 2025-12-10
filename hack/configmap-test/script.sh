# effected versions:
# v1.34.2+k3s1
# v1.33.6+k3s
# v1.32.10
# This does not happen on kind built with latest kubernetes, leading me to believe it is a k3s issue.
# This doesn't happen if the command doesn't use sqllite.  curl -sfL https://get.k3s.io | sh -s - --cluster-init
loop_count=0
while true; do
  loop_count=$((loop_count + 1))
  echo "Loop iteration: $loop_count"
  k3d cluster delete
  k3d cluster create --image rancher/k3s:v1.32.10-k3s1  || continue
  go run main.go || break
done
