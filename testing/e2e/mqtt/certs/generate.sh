#!/bin/bash
# Generate self-signed certificates for EMQX MQTT TLS testing
# These certificates are for testing only - NOT for production use!

set -e

CERTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Generating self-signed certificates for MQTT TLS testing..."

# Generate CA key and certificate
openssl genrsa -out "$CERTS_DIR/ca.key" 2048
openssl req -new -x509 -days 3650 -key "$CERTS_DIR/ca.key" \
    -out "$CERTS_DIR/ca.pem" \
    -subj "/CN=shoutrrr-test-ca/O=Shoutrrr/C=US"

# Generate server key
openssl genrsa -out "$CERTS_DIR/key.pem" 2048

# Generate server certificate request with localhost SAN
cat > "$CERTS_DIR/server.cnf" <<EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn
req_extensions = v3_req

[dn]
C = US
O = Shoutrrr
CN = localhost

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = emqx
IP.1 = 127.0.0.1
IP.2 = 0.0.0.0
EOF

openssl req -new -key "$CERTS_DIR/key.pem" \
    -out "$CERTS_DIR/server.csr" \
    -config "$CERTS_DIR/server.cnf"

# Sign the server certificate with CA
openssl x509 -req -days 365 \
    -in "$CERTS_DIR/server.csr" \
    -CA "$CERTS_DIR/ca.pem" \
    -CAkey "$CERTS_DIR/ca.key" \
    -CAcreateserial \
    -out "$CERTS_DIR/cert.pem" \
    -extensions v3_req \
    -extfile "$CERTS_DIR/server.cnf"

# Set permissions
chmod 600 "$CERTS_DIR"/*.key
chmod 644 "$CERTS_DIR"/*.pem

# Cleanup
rm -f "$CERTS_DIR/ca.key" "$CERTS_DIR/server.csr" "$CERTS_DIR/server.cnf" "$CERTS_DIR/ca.srl"

echo "Certificates generated successfully!"
echo "  - CA: $CERTS_DIR/ca.pem"
echo "  - Server cert: $CERTS_DIR/cert.pem"
echo "  - Server key: $CERTS_DIR/key.pem"
echo ""
echo "To use with EMQX, mount these files and configure:"
echo "  EMQX_LISTENERS__SSL__DEFAULT__SSL_OPTIONS__CACERTFILE=/opt/emqx/etc/certs/ca.pem"
echo "  EMQX_LISTENERS__SSL__DEFAULT__SSL_OPTIONS__CERTFILE=/opt/emqx/etc/certs/cert.pem"
echo "  EMQX_LISTENERS__SSL__DEFAULT__SSL_OPTIONS__KEYFILE=/opt/emqx/etc/certs/key.pem"
