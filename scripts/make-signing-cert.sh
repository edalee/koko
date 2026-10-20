#!/bin/bash
# Creates a self-signed code-signing certificate in your login keychain.
# `make build` signs Koko.app with it, so macOS recognises each local build as
# the same app and keeps the privacy permissions you gave Koko.
# Run it once, with `make signing-cert`. The name is the first argument.
set -euo pipefail

name="${1:-Koko Local}"
keychain="$HOME/Library/Keychains/login.keychain-db"

dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT

# codesign refuses a certificate macOS does not trust for code signing. This
# step asks for your login password.
trust() {
  security add-trusted-cert -r trustRoot -p codeSign -k "$keychain" "$1"
}

if security find-identity -v -p codesigning | grep -qF "\"$name\""; then
  echo "The \"$name\" certificate is already in your keychain and trusted."
  exit 0
fi

# A run that stopped at the password prompt left the certificate untrusted.
if security find-certificate -c "$name" >/dev/null 2>&1; then
  security find-certificate -c "$name" -p >"$dir/cert.pem"
  trust "$dir/cert.pem"
  echo "Trusted the \"$name\" certificate. Run make build to sign Koko with it."
  exit 0
fi

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

security import "$dir/cert.p12" -k "$keychain" -P "$pass" -T /usr/bin/codesign
trust "$dir/cert.pem"

echo "Created the \"$name\" certificate. Run make build to sign Koko with it."
