#!/usr/bin/env bash
# Run E2E scripts in a Linux (amd64) container similar to GitHub Actions.
# Usage: from repo root — ./test/e2e/docker-run.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE="${E2E_DOCKER_IMAGE:-golang:1.26-bookworm}"

docker run --rm --platform linux/amd64 \
	-e GITHUB_WORKSPACE=/work \
	-e CI=1 \
	-v "${REPO_ROOT}:/work" \
	-w /work \
	"${IMAGE}" \
	bash -ceuo pipefail '
apt-get update -qq
apt-get install -y -qq age ca-certificates curl git

curl -fsSL https://github.com/getsops/sops/releases/download/v3.10.2/sops-v3.10.2.linux.amd64 \
	-o /usr/local/bin/sops
chmod +x /usr/local/bin/sops

go build -o ./bin/envdesk ./cmd/envdesk
chmod +x ./test/e2e/*.sh ./test/e2e/lib/*.sh 2>/dev/null || true

./test/e2e/smoke.sh
./test/e2e/mutations.sh
./test/e2e/failures.sh
echo "OK: all e2e scripts passed in container (linux/amd64)"
'

printf 'OK: e2e docker run complete (%s, linux/amd64)\n' "${IMAGE}"
