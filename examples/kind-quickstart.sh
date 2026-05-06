#!/usr/bin/env bash
# End-to-end: spin up a kind cluster, install kagent v0.9.1, deploy
# vps-mcp, register it as a RemoteMCPServer, and apply the vps-devops
# Agent. After this, open the kagent UI to chat.
#
# Prereqs: docker, kind, kubectl, helm, an SSH key with VPS access.
# Required env: VPS_HOST=user@your.vps.ip  VPS_SSH_KEY_PATH=/path/to/key

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Load REPO_ROOT/.env if present (anthropic_api_key, optionally
# VPS_HOST and VPS_SSH_KEY_PATH).
if [[ -f "${REPO_ROOT}/.env" ]]; then
  set -a; . "${REPO_ROOT}/.env"; set +a
fi

# Allow either lower- or upper-case keys in the .env.
ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-${anthropic_api_key:-}}"

: "${VPS_HOST:?Set VPS_HOST=user@your.vps.ip in .env or env}"
: "${VPS_SSH_KEY_PATH:?Set VPS_SSH_KEY_PATH to the private key path}"
: "${ANTHROPIC_API_KEY:?Set anthropic_api_key in .env}"

CLUSTER_NAME="${CLUSTER_NAME:-vps-mcp-demo}"
# Charts publish without the "v" prefix on the OCI tag.
KAGENT_VERSION="${KAGENT_VERSION:-0.9.1}"
KAGENT_NAMESPACE="${KAGENT_NAMESPACE:-kagent}"
KAGENT_CRDS_CHART="oci://ghcr.io/kagent-dev/kagent/helm/kagent-crds"
KAGENT_CHART="oci://ghcr.io/kagent-dev/kagent/helm/kagent"

echo "==> Creating kind cluster '${CLUSTER_NAME}' (skipping if exists)"
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
  kind create cluster --name "${CLUSTER_NAME}"
fi

echo "==> Installing kagent ${KAGENT_VERSION} (OCI charts)"

# CRDs first.
helm upgrade --install kagent-crds "${KAGENT_CRDS_CHART}" \
  --namespace "${KAGENT_NAMESPACE}" --create-namespace \
  --version "${KAGENT_VERSION}"

# Anthropic API key Secret — created before the controller so the
# default ModelConfig finds it on first reconcile.
kubectl create secret generic kagent-anthropic \
  --namespace "${KAGENT_NAMESPACE}" \
  --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

# Controller, with Anthropic as the default provider.
helm upgrade --install kagent "${KAGENT_CHART}" \
  --namespace "${KAGENT_NAMESPACE}" \
  --version "${KAGENT_VERSION}" \
  --set providers.default=anthropic

echo "==> Building vps-mcp image and loading into kind"
docker build -t vps-mcp:dev -f "${REPO_ROOT}/deploy/Dockerfile" "${REPO_ROOT}"
kind load docker-image vps-mcp:dev --name "${CLUSTER_NAME}"

echo "==> Creating vps-mcp Secret"
kubectl create secret generic vps-mcp \
  --namespace "${KAGENT_NAMESPACE}" \
  --from-literal=host="${VPS_HOST}" \
  --from-file=ssh_key="${VPS_SSH_KEY_PATH}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> Applying Deployment + Service"
sed 's#ghcr.io/bufordeeds/vps-mcp:latest#vps-mcp:dev#' \
  "${REPO_ROOT}/deploy/kubernetes/deployment.yaml" \
  | kubectl apply -n "${KAGENT_NAMESPACE}" -f -
kubectl apply -n "${KAGENT_NAMESPACE}" -f "${REPO_ROOT}/deploy/kubernetes/service.yaml"

echo "==> Waiting for vps-mcp to become Ready"
kubectl wait deploy/vps-mcp --namespace "${KAGENT_NAMESPACE}" \
  --for=condition=Available --timeout=120s

echo "==> Registering vps-mcp as a RemoteMCPServer"
kubectl apply -f "${REPO_ROOT}/deploy/kagent/remotemcpserver.yaml"

echo "==> Applying vps-devops Agent"
kubectl apply -f "${REPO_ROOT}/deploy/kagent/agent.yaml"

cat <<EOF

==> Done.

To open the kagent UI:
  kubectl port-forward -n ${KAGENT_NAMESPACE} svc/kagent-ui 8080:80
  open http://localhost:8080

Then pick the 'vps-devops' agent and try:
  - "Is the VPS healthy?"
  - "Did anyone visit mosscreekdigital.com today?"

To tear down:
  kind delete cluster --name ${CLUSTER_NAME}
EOF
