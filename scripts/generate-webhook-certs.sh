#!/bin/bash
set -e

echo "🔐 Generating webhook certificates for KubeStellar Integration Toolkit..."

# Configuration
NAMESPACE="${NAMESPACE:-ksit-system}"
SERVICE_NAME="${SERVICE_NAME:-ksit-webhook-service}"
SECRET_NAME="${SECRET_NAME:-ksit-webhook-server-cert}"
WEBHOOK_CONFIG_NAME="ksit-validating-webhook-configuration"
CERT_VALIDITY_DAYS=825

echo "📋 Configuration:"
echo "  Namespace:     $NAMESPACE"
echo "  Service:       $SERVICE_NAME"
echo "  Secret:        $SECRET_NAME"
echo "  Validity:      $CERT_VALIDITY_DAYS days"

# Create temporary directory
CERT_DIR=$(mktemp -d)
echo "📁 Working directory: $CERT_DIR"
cd "$CERT_DIR"

# Cleanup function
cleanup() {
    echo "🧹 Cleaning up temporary files..."
    rm -rf "$CERT_DIR"
}
trap cleanup EXIT

# Generate CA private key
echo "🔑 Generating CA private key..."
openssl genrsa -out ca.key 2048

# Generate CA certificate
echo "📜 Generating CA certificate..."
openssl req -x509 -new -nodes -key ca.key -sha256 -days $CERT_VALIDITY_DAYS \
    -out ca.crt \
    -subj "/CN=ksit-webhook-ca/O=kubestellar"

# Generate server private key
echo "🔑 Generating server private key..."
openssl genrsa -out tls.key 2048

# Generate server CSR
echo "📝 Generating server certificate signing request..."
openssl req -new -key tls.key -out server.csr \
    -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc/O=kubestellar"

# Create certificate extensions config
echo "📝 Creating certificate extensions..."
cat > server.ext <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
DNS.4 = ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local
EOF

# Sign server certificate with CA
echo "✍️  Signing server certificate..."
openssl x509 -req -in server.csr \
    -CA ca.crt -CAkey ca.key \
    -CAcreateserial \
    -out tls.crt \
    -days $CERT_VALIDITY_DAYS \
    -sha256 \
    -extfile server.ext

# Verify certificate
echo "✔️  Verifying certificate..."
openssl x509 -in tls.crt -text -noout | grep -A 1 "Subject Alternative Name" || true

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "⚠️  kubectl not found. Skipping Kubernetes secret creation."
    echo "📁 Certificates saved in: $CERT_DIR"
    echo "   ca.crt, tls.crt, tls.key"
    exit 0
fi

# Create namespace if it doesn't exist
echo "🔧 Creating namespace: $NAMESPACE"
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# Create or update Kubernetes secret
echo "🔒 Creating Kubernetes secret: $SECRET_NAME"
kubectl create secret tls ${SECRET_NAME} \
    --cert=tls.crt \
    --key=tls.key \
    --namespace=${NAMESPACE} \
    --dry-run=client -o yaml | kubectl apply -f -

# Get CA bundle for webhook configuration
CA_BUNDLE=$(cat ca.crt | base64 | tr -d '\n')

# Create webhook configuration patch
WEBHOOK_PATCH_FILE="webhook-patch.yaml"
cat > "$WEBHOOK_PATCH_FILE" <<EOF
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: ${WEBHOOK_CONFIG_NAME}
webhooks:
  - name: integrations.ksit.io
    clientConfig:
      caBundle: ${CA_BUNDLE}
      service:
        name: ${SERVICE_NAME}
        namespace: ${NAMESPACE}
        path: "/validate-v1alpha1-integration"
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["ksit.io"]
        apiVersions: ["v1alpha1"]
        resources: ["integrations"]
    admissionReviewVersions: ["v1", "v1beta1"]
    sideEffects: None
    failurePolicy: Fail
  - name: integrationtargets.ksit.io
    clientConfig:
      caBundle: ${CA_BUNDLE}
      service:
        name: ${SERVICE_NAME}
        namespace: ${NAMESPACE}
        path: "/validate-v1alpha1-integrationtarget"
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["ksit.io"]
        apiVersions: ["v1alpha1"]
        resources: ["integrationtargets"]
    admissionReviewVersions: ["v1", "v1beta1"]
    sideEffects: None
    failurePolicy: Fail
EOF

echo ""
echo "✅ Webhook certificates generated successfully!"
echo ""
echo "📋 Summary:"
echo "  Namespace:        $NAMESPACE"
echo "  Secret:           $SECRET_NAME"
echo "  Webhook Config:   $WEBHOOK_CONFIG_NAME"
echo "  CA Certificate:   $CERT_DIR/ca.crt"
echo "  Server Cert:      $CERT_DIR/tls.crt"
echo "  Server Key:       $CERT_DIR/tls.key"
echo ""
echo "📝 CA Bundle (Base64):"
echo "$CA_BUNDLE"
echo ""
echo "📄 Webhook configuration saved to: $WEBHOOK_PATCH_FILE"
echo ""
echo "🚀 To apply webhook configuration:"
echo "  kubectl apply -f $WEBHOOK_PATCH_FILE"
echo ""
echo "🔍 To verify secret:"
echo "  kubectl get secret $SECRET_NAME -n $NAMESPACE"
echo ""
echo "📖 Next steps:"
echo "  1. Update config/webhook/validating_webhook_configuration.yaml with CA bundle"
echo "  2. Deploy webhook: make deploy-webhook"
echo "  3. Test webhook: kubectl apply -f config/samples/"