#!/bin/bash

# Deep TLS validation script for registry mTLS
set -e

echo "=== Deep TLS Validation ==="
echo ""

# Function to check TLS handshake details
check_tls_handshake() {
    local service=$1
    local port=$2

    echo "Checking TLS handshake for $service:$port..."

    # Create a test pod with openssl and curl
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: tls-validator
  namespace: zarf
  labels:
    zarf.dev/agent: ignore
spec:
  containers:
  - name: validator
    image: curlimages/curl:latest
    command: ["/bin/sh", "-c", "sleep 3600"]
    volumeMounts:
    - name: ca-cert
      mountPath: /certs/ca
      readOnly: true
    - name: client-cert
      mountPath: /certs/client
      readOnly: true
  volumes:
  - name: ca-cert
    secret:
      secretName: zarf-registry-ca
  - name: client-cert
    secret:
      secretName: zarf-registry-proxy-tls
  restartPolicy: Never
EOF

    kubectl wait --for=condition=Ready pod/tls-validator -n zarf --timeout=60s

    # Test the TLS connection with detailed output
    echo "Testing TLS handshake with verbose output..."
    kubectl exec -n zarf tls-validator -- sh -c "
        echo 'Testing basic TLS connection...'
        timeout 10 openssl s_client -connect $service:$port -servername $service -verify 1 -CAfile /certs/ca/ca.pem -cert /certs/client/tls.crt -key /certs/client/tls.key -brief 2>&1 | head -20
    "

    echo ""
    echo "Testing with curl (should show TLS details)..."
    kubectl exec -n zarf tls-validator -- sh -c "
        curl -v --cacert /certs/ca/ca.pem --cert /certs/client/tls.crt --key /certs/client/tls.key https://$service:$port/v2/ 2>&1 | grep -E '(SSL|TLS|certificate|handshake|verify)'
    "

    kubectl delete pod tls-validator -n zarf
}

# Function to analyze certificates
analyze_certificates() {
    echo "=== Certificate Analysis ==="

    # Extract and analyze CA certificate
    echo "CA Certificate Details:"
    kubectl get secret zarf-registry-ca -n zarf -o jsonpath='{.data.ca\.pem}' | base64 -d > /tmp/ca.pem
    openssl x509 -in /tmp/ca.pem -text -noout | grep -A5 -B5 "Subject:\|Issuer:\|Not Before:\|Not After:"
    echo ""

    # Extract and analyze server certificate
    echo "Server Certificate Details:"
    kubectl get secret zarf-registry-server-tls -n zarf -o jsonpath='{.data.tls\.crt}' | base64 -d > /tmp/server.pem
    openssl x509 -in /tmp/server.pem -text -noout | grep -A10 -B5 "Subject:\|Issuer:\|DNS:\|IP Address:\|Not Before:\|Not After:"
    echo ""

    # Verify certificate chain
    echo "Verifying certificate chain..."
    openssl verify -CAfile /tmp/ca.pem /tmp/server.pem
    echo ""

    # Check certificate purposes
    echo "Certificate purposes:"
    openssl x509 -in /tmp/server.pem -purpose -noout
    echo ""

    # Clean up temp files
    rm -f /tmp/ca.pem /tmp/server.pem
}

# Function to monitor network traffic
monitor_traffic() {
    echo "=== Network Traffic Monitoring ==="

    local proxy_pod=$(kubectl get pods -n zarf -l app=zarf-registry-proxy -o jsonpath='{.items[0].metadata.name}')
    local registry_pod=$(kubectl get pods -n zarf -l app=docker-registry -o jsonpath='{.items[0].metadata.name}')

    if [ -n "$proxy_pod" ] && [ -n "$registry_pod" ]; then
        echo "Setting up network monitoring..."
        echo "Proxy pod: $proxy_pod"
        echo "Registry pod: $registry_pod"

        # Check if we can install tcpdump
        echo "Checking network capabilities..."

        # Alternative: Use netstat to check connections
        echo "Current network connections on proxy:"
        kubectl exec -n zarf $proxy_pod -- netstat -an 2>/dev/null | grep :5000 || echo "No connections to port 5000 found"

        echo ""
        echo "Current network connections on registry:"
        kubectl exec -n zarf $registry_pod -- netstat -an 2>/dev/null | grep :5000 || echo "No connections on port 5000 found"

    else
        echo "Could not find proxy or registry pods"
    fi
}

# Function to test the actual mTLS flow
test_mtls_flow() {
    echo "=== Testing mTLS Flow ==="

    # Create a comprehensive test pod
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: mtls-test
  namespace: zarf
  labels:
    zarf.dev/agent: ignore
spec:
  containers:
  - name: test
    image: alpine:latest
    command: ["/bin/sh", "-c", "apk add --no-cache openssl curl && sleep 3600"]
    volumeMounts:
    - name: ca-cert
      mountPath: /certs/ca
      readOnly: true
    - name: client-cert
      mountPath: /certs/client
      readOnly: true
  volumes:
  - name: ca-cert
    secret:
      secretName: zarf-registry-ca
  - name: client-cert
    secret:
      secretName: zarf-registry-proxy-tls
  restartPolicy: Never
EOF

    kubectl wait --for=condition=Ready pod/mtls-test -n zarf --timeout=60s

    echo "Testing mTLS connection with full handshake details..."
    kubectl exec -n zarf mtls-test -- sh -c '
        echo "=== Full TLS Handshake Test ==="
        echo "GET /v2/" | openssl s_client -connect docker-registry:5000 -CAfile /certs/ca/ca.pem -cert /certs/client/tls.crt -key /certs/client/tls.key -verify 1 -servername docker-registry -debug 2>&1 | head -50
    '

    echo ""
    echo "Testing certificate verification..."
    kubectl exec -n zarf mtls-test -- sh -c '
        echo "=== Certificate Verification ==="
        openssl verify -CAfile /certs/ca/ca.pem /certs/client/tls.crt
        echo "Client certificate verification complete"
    '

    echo ""
    echo "Testing actual registry API call..."
    kubectl exec -n zarf mtls-test -- sh -c '
        echo "=== Registry API Test ==="
        curl -v --cacert /certs/ca/ca.pem --cert /certs/client/tls.crt --key /certs/client/tls.key https://docker-registry:5000/v2/ 2>&1 | grep -E "(TLS|SSL|certificate|Connected)"
    '

    kubectl delete pod mtls-test -n zarf
}

# Main execution
echo "Starting comprehensive TLS validation..."
echo "========================================"

# 1. Analyze certificates
analyze_certificates

# 2. Check TLS handshake
check_tls_handshake "zarf-docker-registry" "5000"

# 3. Monitor traffic
monitor_traffic

# 4. Test mTLS flow
test_mtls_flow

echo ""
echo "=== Validation Summary ==="
echo "1. Check the logs above for any TLS handshake errors"
echo "2. Look for 'Verify return code: 0 (ok)' in openssl output"
echo "3. Ensure curl shows 'SSL connection using TLS'"
echo "4. Registry should respond with HTTP 200 or 401 (auth required)"
echo ""
echo "If you see any certificate verification errors, the mTLS setup may need adjustment."
