#!/bin/bash

wget -q "https://umbrella-bigbang-releases.s3-us-gov-west-1.amazonaws.com/umbrella/${1}/package-images.yaml" -O "images.yaml"

yq .package-image-list < "images.yaml" \
  | sed -e 's/- //g' \
  | sed -e ':r;$!{N;br};s/\n  images:/] = []string{/g' \
  | sed -E ':r;$!{N;br};s/(\n)(\S)/\n}\1BigBangImages["\2/g' \
  | sed -e ':r;$!{N;br};s/\n  version://g' \
  | sed -e 's/    /  /g' \
  | sed -e 's/: "/"]["/g' \
  | sed -E ':r;$!{N;br};s/(")(\n)/\1,\2/g' \
  | sed -e 's/istio"/istio-controlplane"/g' \
  | sed -e 's/"istiooperator"/"istio-operator"/g' \
  | sed -e 's/"kyvernopolicies"/"kyverno-policies"/g' \
  | sed -e 's/"gatekeeper"/"policy"/g' \
  | sed -e 's/"clusterAuditor"/"cluster-auditor"/g' \
  | sed -e 's/"eckoperator"/"eck-operator"/g' \
  | sed -e 's/"logging"/"elasticsearch-kibana"/g' \
  | sed -e 's/"metricsServer"/"metrics-server"/g' > "images.${1}.go"

echo '}' >> "images.${1}.go"

echo -e "BigBangImages[\"$(cat images.${1}.go)" > "images.${1}.go"

cat "images.${1}.go"

rm images.yaml