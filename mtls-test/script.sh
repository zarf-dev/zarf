#!/bin/bash

# Create TLS certificates for Zarf Registry
# This script creates all the necessary certificates for the registry and proxy

set -e

# Create a temporary directory for certificates
CERT_DIR="cert_dir"
mkdir -p $CERT_DIR
echo "Working in directory: $CERT_DIR"
cd "$CERT_DIR"

# 1. Create CA private key
echo "Creating CA private key..."
openssl genrsa -out ca-key.pem 4096

# 2. Create CA certificate
echo "Creating CA certificate..."
openssl req -new -x509 -days 365 -key ca-key.pem -sha256 -out ca.pem -subj "/C=US/ST=CA/L=San Francisco/O=Zarf/CN=Zarf Registry CA"

# 3. Create server private key
echo "Creating server private key..."
openssl genrsa -out server-key.pem 4096

# 4. Create server certificate signing request
echo "Creating server CSR..."
openssl req -subj "/C=US/ST=CA/L=San Francisco/O=Zarf/CN=zarf-registry" -sha256 -new -key server-key.pem -out server.csr

# 5. Create server certificate extensions
echo "Creating server certificate extensions..."
cat > server-extfile.cnf <<EOF
subjectAltName = DNS:zarf-docker-registry,DNS:localhost,IP:127.0.0.1
extendedKeyUsage = serverAuth
EOF

# 6. Generate server certificate
echo "Generating server certificate..."
openssl x509 -req -days 365 -sha256 -in server.csr -CA ca.pem -CAkey ca-key.pem -out server-cert.pem -extfile server-extfile.cnf -CAcreateserial

# 7. Create client private key (for proxy)
echo "Creating client private key..."
openssl genrsa -out client-key.pem 4096

# 8. Create client certificate signing request
echo "Creating client CSR..."
openssl req -subj "/C=US/ST=CA/L=San Francisco/O=Zarf/CN=zarf-registry-proxy" -new -key client-key.pem -out client.csr

# 9. Create client certificate extensions
echo "Creating client certificate extensions..."
cat > client-extfile.cnf <<EOF
extendedKeyUsage = clientAuth
EOF

# 10. Generate client certificate
echo "Generating client certificate..."
openssl x509 -req -days 365 -sha256 -in client.csr -CA ca.pem -CAkey ca-key.pem -out client-cert.pem -extfile client-extfile.cnf -CAcreateserial

# 11. Set proper permissions
chmod 400 ca-key.pem server-key.pem client-key.pem
chmod 444 ca.pem server-cert.pem client-cert.pem

echo "Certificates created successfully!"

# 12. Create Kubernetes secrets
echo "Creating Kubernetes secrets..."

# Create namespace if it doesn't exist
kubectl create namespace zarf --dry-run=client -o yaml | kubectl apply -f -

# Create CA secret
kubectl create secret generic zarf-registry-ca \
  --from-file=ca.pem=ca.pem \
  --namespace=zarf \
  --dry-run=client -o yaml | kubectl apply -f -

# Create server TLS secret
kubectl create secret tls zarf-registry-server-tls \
  --cert=server-cert.pem \
  --key=server-key.pem \
  --namespace=zarf \
  --dry-run=client -o yaml | kubectl apply -f -

# Create proxy TLS secret
kubectl create secret tls zarf-registry-proxy-tls \
  --cert=client-cert.pem \
  --key=client-key.pem \
  --namespace=zarf \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Kubernetes secrets created successfully!"

# 13. Verify secrets
echo "Verifying secrets..."
kubectl get secrets -n zarf | grep zarf-registry
