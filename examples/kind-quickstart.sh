#!/usr/bin/env bash
# End-to-end quickstart: spin up a kind cluster, install kagent, deploy
# vps-mcp, and apply an example agent. This is the script form of the
# README's "demo" section.
#
# Prereqs: docker, kind, kubectl, helm, an SSH key with access to your VPS.
# Set VPS_HOST and VPS_SSH_KEY_PATH before running.

set -euo pipefail

: "${VPS_HOST:?Set VPS_HOST=user@your.vps.ip}"
: "${VPS_SSH_KEY_PATH:?Set VPS_SSH_KEY_PATH to the private key path}"

CLUSTER_NAME="${CLUSTER_NAME:-vps-mcp-demo}"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "==> Creating kind cluster '${CLUSTER_NAME}' (skipping if exists)"
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
  kind create cluster --name "${CLUSTER_NAME}"
fi

echo "==> Building vps-mcp image and loading into kind"
docker build -t vps-mcp:dev -f "${REPO_ROOT}/deploy/Dockerfile" "${REPO_ROOT}"
kind load docker-image vps-mcp:dev --name "${CLUSTER_NAME}"

echo "==> Creating vps-mcp secret"
kubectl create secret generic vps-mcp \
  --from-literal=host="${VPS_HOST}" \
  --from-file=ssh_key="${VPS_SSH_KEY_PATH}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> Applying Deployment"
# Pin to the locally built image instead of GHCR.
sed 's#ghcr.io/bufordeeds/vps-mcp:latest#vps-mcp:dev#' \
  "${REPO_ROOT}/deploy/kubernetes/deployment.yaml" | kubectl apply -f -

echo "==> Waiting for vps-mcp to become Ready"
kubectl wait deploy/vps-mcp --for=condition=Available --timeout=120s

cat <<EOF

==> Done.

Next steps:
  1. Install kagent (see https://kagent.dev/docs/getting-started)
  2. Apply ${REPO_ROOT}/deploy/kagent/agent.yaml.example (after editing
     for your kagent version)
  3. Open the kagent UI and ask "Is the VPS healthy?"

To tear down:
  kind delete cluster --name ${CLUSTER_NAME}
EOF
