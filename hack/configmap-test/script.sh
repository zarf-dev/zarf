# this happens for sure on v1.32.10
# v1.32.8 and v1.32.9 is unsure, but fairly likely okay
# v1.34.2+k3s1
loop_count=0
while true; do
  loop_count=$((loop_count + 1))
  echo "Loop iteration: $loop_count"
  k3d cluster delete
  k3d cluster create --image rancher/k3s:v1.32.10-k3s1  || continue
  go run main.go || break
done
