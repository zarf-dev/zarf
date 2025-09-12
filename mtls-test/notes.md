Manually setting up mTLS
- Create certs for both the registry and the proxy
- Create certificate signing requests in Kubernetes. Unless a user specifies the certificate is going to be self signed by the tool that makes the cluster. For example, this documents the k3s process of making self signed certs https://docs.k3s.io/cli/certificate. Because of this, when zarf pushes images to the registry and does a port forward it won't be able to connect over TLS assuming that the cluster was not given specific certificates to use at it's CA that the host of Zarf is already using.

To does bring me to question the value of the feature. Users will definitely have



# Cluster setup


openssl genrsa -out /certs/proxy/key.pem 2048
openssl req -new -key /certs/proxy/key.pem -out /certs/proxy/csr.pem -subj "/CN=zarf-registry-proxy"

# Generate registry server certificate
cat > /certs/registry/csr.conf <<EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn
req_extensions = v3_req

[dn]
CN=zarf-docker-registry

[v3_req]
basicConstraints = CA:FALSE
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = zarf-docker-registry
DNS.2 = zarf-docker-registry.zarf.svc.cluster.local
IP.1 = 127.0.0.1
EOF

openssl genrsa -out /certs/registry/key.pem 2048
openssl req -new -key /certs/registry/key.pem -out /certs/registry/csr.pem -config /certs/registry/csr.conf

# Create CSRs in Kubernetes
cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: zarf-registry-proxy-csr
spec:
  request: $(cat /certs/proxy/csr.pem | base64 | tr -d '\n')
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - client auth
EOF

cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: zarf-registry-server-csr
spec:
  request: $(cat /certs/registry/csr.pem | base64 | tr -d '\n')
  signerName: kubernetes.io/kubelet-serving
  usages:
  - server auth
EOF

# Wait for CSRs to be created
sleep 5

# Auto-approve CSRs (in production, you might want manual approval)
kubectl certificate approve zarf-registry-proxy-csr
kubectl certificate approve zarf-registry-server-csr

# Wait for certificates to be issued
sleep 10

# Get the issued certificates
kubectl get csr zarf-registry-proxy-csr -o jsonpath='{.status.certificate}' | base64 -d > /certs/proxy/cert.pem
kubectl get csr zarf-registry-server-csr -o jsonpath='{.status.certificate}' | base64 -d > /certs/registry/cert.pem

# Get the Kubernetes cluster CA certificate for validation
kubectl get configmap kube-root-ca.crt -o jsonpath='{.data.ca\.crt}' > /certs/ca.pem

# Create secrets with certificates
kubectl create secret tls zarf-registry-proxy-tls \
  --cert=/certs/proxy/cert.pem \
  --key=/certs/proxy/key.pem \
  --namespace=zarf

kubectl create secret tls zarf-registry-server-tls \
  --cert=/certs/registry/cert.pem \
  --key=/certs/registry/key.pem \
  --namespace=zarf

# Create CA secret for verification (using Kubernetes cluster CA)
kubectl create secret generic zarf-registry-ca \
  --from-file=ca.pem=/certs/ca.pem \
  --namespace=zarf
