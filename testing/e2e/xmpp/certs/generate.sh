#!/bin/bash
# Generate self-signed certificates for ejabberd TLS testing.
# These certificates are for testing only.

set -euo pipefail

CERTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Generating self-signed certificates for XMPP TLS testing..."

openssl genrsa -out "$CERTS_DIR/ca.key" 2048
openssl req -new -x509 -days 3650 -key "$CERTS_DIR/ca.key" \
    -out "$CERTS_DIR/ca.pem" \
    -subj "/CN=shoutrrr-xmpp-test-ca/O=Shoutrrr/C=US"

openssl genrsa -out "$CERTS_DIR/key.pem" 2048

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
DNS.2 = ejabberd
IP.1 = 127.0.0.1
IP.2 = 0.0.0.0
EOF

openssl req -new -key "$CERTS_DIR/key.pem" \
    -out "$CERTS_DIR/server.csr" \
    -config "$CERTS_DIR/server.cnf"

openssl x509 -req -days 3650 \
    -in "$CERTS_DIR/server.csr" \
    -CA "$CERTS_DIR/ca.pem" \
    -CAkey "$CERTS_DIR/ca.key" \
    -CAcreateserial \
    -out "$CERTS_DIR/cert.pem" \
    -extensions v3_req \
    -extfile "$CERTS_DIR/server.cnf"

cat "$CERTS_DIR/cert.pem" "$CERTS_DIR/key.pem" > "$CERTS_DIR/server.pem"

chmod 644 "$CERTS_DIR"/*.pem
rm -f "$CERTS_DIR/ca.key" "$CERTS_DIR/server.csr" "$CERTS_DIR/server.cnf" "$CERTS_DIR/ca.srl"

echo "Certificates generated successfully."
