#!/usr/bin/env bash
# Нагрузочный тест: отправляет много запросов подряд и показывает, сколько из них
# получили 429 (превышен RPS лимит subdomain) и другие ошибки прокси.
set -euo pipefail

PROXY_URL="${PROXY_URL:-http://localhost:8080}"
SUBDOMAIN="${SUBDOMAIN:-timconsult}"
JWT_ISSUER="${JWT_ISSUER:-local-debug}"
JWT_AUDIENCE="${JWT_AUDIENCE:-local-debug}"
JWT_PRIVATE_KEY_FILE="${JWT_PRIVATE_KEY_FILE:-.jwt-private.debug.pem}"

# Количество запросов и параллелизм задаются извне: превышают лимит RPS subdomain,
# чтобы часть запросов получила 429.
TOTAL_REQUESTS="${TOTAL_REQUESTS:-50}"
CONCURRENCY="${CONCURRENCY:-20}"

# Генерирует RS256 JWT с claim subdomain для локального теста (один токен на весь прогон).
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

# Выполняет один запрос и печатает только HTTP-статус.
request_status() {
  curl -sS -o /dev/null -w "%{http_code}\n" \
    -H "Authorization: Bearer ${JWT}" \
    "${PROXY_URL}/${SUBDOMAIN}/api/v4/account"
}
export -f request_status
export JWT PROXY_URL SUBDOMAIN

echo "Отправка ${TOTAL_REQUESTS} запросов с параллелизмом ${CONCURRENCY}..."

STATUSES_FILE=$(mktemp)
trap 'rm -f "$STATUSES_FILE"' EXIT

seq 1 "$TOTAL_REQUESTS" | xargs -P "$CONCURRENCY" -I{} bash -c 'request_status' > "$STATUSES_FILE"

echo
echo "Распределение статусов:"
sort "$STATUSES_FILE" | uniq -c | sort -rn

TOTAL=$(wc -l < "$STATUSES_FILE")
OK=$(grep -c '^200$' "$STATUSES_FILE" || true)
RATE_LIMITED=$(grep -c '^429$' "$STATUSES_FILE" || true)
OTHER=$((TOTAL - OK - RATE_LIMITED))

echo
echo "Всего: ${TOTAL}, успешно (200): ${OK}, ограничено лимитом (429): ${RATE_LIMITED}, другие ошибки: ${OTHER}"
