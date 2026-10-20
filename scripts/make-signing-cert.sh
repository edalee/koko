#!/bin/bash
# Creates a self-signed code-signing certificate in your login keychain.
# `make build` signs Koko.app with it, so every local build has the same
# signature and macOS keeps the privacy permissions you gave Koko.
# Run it once, with `make signing-cert`. The name is the first argument.
set -euo pipefail

name="${1:-Koko Local}"

if security find-certificate -c "$name" >/dev/null 2>&1; then
  echo "The \"$name\" certificate is already in your keychain."
  exit 0
fi

dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT

cat >"$dir/cert.conf" <<EOF
[req]
distinguished_name = dn
x509_extensions = ext
prompt = no
[dn]
CN = $name
[ext]
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature
extendedKeyUsage = critical,codeSigning
EOF

# The system LibreSSL writes a PKCS#12 file the keychain can read. OpenSSL 3
# from Homebrew uses a cipher the keychain rejects.
/usr/bin/openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
  -config "$dir/cert.conf" -keyout "$dir/key.pem" -out "$dir/cert.pem" 2>/dev/null

# The password only carries the key into the keychain, and is never shown.
pass=$(/usr/bin/openssl rand -hex 16)
/usr/bin/openssl pkcs12 -export -name "$name" -inkey "$dir/key.pem" -in "$dir/cert.pem" \
  -passout "pass:$pass" -out "$dir/cert.p12"

security import "$dir/cert.p12" -k "$HOME/Library/Keychains/login.keychain-db" \
  -P "$pass" -T /usr/bin/codesign

# codesign refuses a certificate macOS does not trust for code signing. This
# step asks for your login password.
security add-trusted-cert -r trustRoot -p codeSign \
  -k "$HOME/Library/Keychains/login.keychain-db" "$dir/cert.pem"

echo "Created the \"$name\" certificate. Run make build to sign Koko with it."
