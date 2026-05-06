#!/usr/bin/env bash
# End-to-end: spin up a kind cluster, install kagent v0.9.1, deploy
# vps-mcp, register it as a RemoteMCPServer, and apply the vps-devops
# Agent. After this, open the kagent UI to chat.
#
# Prereqs: docker, kind, kubectl, helm, an SSH key with VPS access.
# Required env: VPS_HOST=user@your.vps.ip  VPS_SSH_KEY_PATH=/path/to/key

set -euo pipefail

: "${VPS_HOST:?Set VPS_HOST=user@your.vps.ip}"
: "${VPS_SSH_KEY_PATH:?Set VPS_SSH_KEY_PATH to the private key path}"

CLUSTER_NAME="${CLUSTER_NAME:-vps-mcp-demo}"
KAGENT_VERSION="${KAGENT_VERSION:-v0.9.1}"
KAGENT_NAMESPACE="${KAGENT_NAMESPACE:-kagent}"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "==> Creating kind cluster '${CLUSTER_NAME}' (skipping if exists)"
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
  kind create cluster --name "${CLUSTER_NAME}"
fi

echo "==> Installing kagent ${KAGENT_VERSION}"
helm repo add kagent https://kagent-dev.github.io/kagent/helm 2>/dev/null || true
helm repo update kagent
# CRDs first.
helm upgrade --install kagent-crds kagent/kagent-crds \
  --namespace "${KAGENT_NAMESPACE}" --create-namespace \
  --version "${KAGENT_VERSION}"
# Then the controller. You will be prompted to provide a model API key
# (Anthropic or OpenAI) when this completes — see kagent's docs.
helm upgrade --install kagent kagent/kagent \
  --namespace "${KAGENT_NAMESPACE}" \
  --version "${KAGENT_VERSION}"

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
