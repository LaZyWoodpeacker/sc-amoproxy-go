#!/usr/bin/env bash
# Запрашивает параметры аккаунта amoCRM через локальный прокси для subdomain "timconsult".
set -euo pipefail

# PROXY_URL="${PROXY_URL:-http://localhost:8080}"
PROXY_URL="https://bbar385mrh3njaif51dn.containers.yandexcloud.net"
SUBDOMAIN="timconsult"
JWT_ISSUER="${JWT_ISSUER:-local-debug}"
JWT_AUDIENCE="${JWT_AUDIENCE:-local-debug}"
JWT_PRIVATE_KEY_FILE="${JWT_PRIVATE_KEY_FILE:-.jwt-private.debug.pem}"

# Генерирует RS256 JWT с claim subdomain для локального теста.
JWT=$(python3 - "$JWT_PRIVATE_KEY_FILE" "$JWT_ISSUER" "$JWT_AUDIENCE" "$SUBDOMAIN" <<'PY'
import base64, json, sys, time
from subprocess import run, PIPE

key_file, issuer, audience, subdomain = sys.argv[1:5]

def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()

header = {"alg": "RS256", "typ": "JWT"}
now = int(time.time())
payload = {
    "iss": issuer,
    "aud": audience,
    "subdomain": subdomain,
    "iat": now,
    "nbf": now,
    "exp": now + 300,
}

signing_input = b".".join([
    b64url(json.dumps(header, separators=(",", ":")).encode()).encode(),
    b64url(json.dumps(payload, separators=(",", ":")).encode()).encode(),
])

result = run(
    ["openssl", "dgst", "-sha256", "-sign", key_file],
    input=signing_input, stdout=PIPE, check=True,
)

print((signing_input + b"." + b64url(result.stdout).encode()).decode())
PY
)

curl -sS -i \
  -H "Authorization: Bearer ${JWT}" \
  "${PROXY_URL}/${SUBDOMAIN}/api/v4/account"
