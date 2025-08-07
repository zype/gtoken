# ECR Token Management in gtoken-webhook

This document outlines the implementation of ECR token management within the gtoken-webhook, enabling secure access to Amazon ECR from GKE pods.

## Overview

The webhook manages ECR tokens by:
1. Using its own GCP credentials to assume an AWS role
2. Using those credentials to get ECR tokens
3. Managing a master ECR secret in the gtoken namespace
4. Copying the secret to namespaces as needed
5. Refreshing tokens before expiration

## Webhook Admission Requirements

The webhook enforces the following requirements during pod admission:

1. ECR Image Detection:
   - Scans all container images in the pod spec
   - Identifies images matching ECR registry pattern (*.dkr.ecr.*.amazonaws.com/*)
   - Denies admission if ECR images are found but no valid pull secret exists

2. Secret Validation:
   - Verifies the ECR pull secret exists in the target namespace
   - Ensures the secret is properly configured with valid credentials
   - Denies admission if secret validation fails

3. Pod Configuration:
   - Ensures the pod's imagePullSecrets references the ECR secret
   - Denies admission if the secret reference is missing

The webhook will return a Denied response if any of these requirements are not met, preventing pods with ECR images from being created without proper pull credentials.

## Architecture

### Webhook Startup

When the webhook starts:
1. Get own GCP service account email from metadata server
2. Generate ID token using IAM credentials API
3. Use token to assume AWS role for ECR access
4. Store AWS credentials and their expiration time
5. Get ECR token using AWS credentials
6. Store ECR token and its expiration time
7. Start background goroutine to:
    - Get token duration from JWT
    - Get AWS credentials expiration from AssumeRoleWithWebIdentity
    - Get ECR token expiration from GetAuthorizationToken
    - Calculate refresh interval:
        - min(nextExpiration - 60s, max(5m, calculatedRefreshInterval))
    - When refreshing:
        - If GCP ID token expires: refresh everything
        - If AWS credentials expire: refresh AWS and ECR
        - If only ECR expires: just refresh ECR
    - Update master ECR secret in gtoken namespace
    - Update ConfigMap with token hash/timestamp
    - Find all namespaces with copied ECR secrets (using label)
    - Update each copied secret with new token
    - Add ecr-token-version annotation to track changes

### Pod Admission

When a pod creation request comes in:
1. Check for ECR images (matching *.dkr.ecr.*.amazonaws.com/*)
2. If ECR images found:
    - Copy master ECR secret to pod's namespace if needed:
        - Use Kubernetes API create/update operations for idempotency
        - Label namespace with gtoken.doit-intl.com/ecr-managed: "true"
        - Retry with exponential backoff on transient failures
        - If retries fail, deny admission - pod cannot start without ECR credentials
    - Add secret to pod's imagePullSecrets
3. If pod has AWS role annotation:
    - Continue with existing sidecar injection logic
    - Let sidecar handle AWS credentials for pod's containers

## Health Checks

### /healthz
- ECR token is active
- AWS credentials are valid
- Secret content hash matches in-memory token
- Check ecr-token-version annotation
- Verify ConfigMap state

### /selftest
- Full STS credential validation
- ECR push/pull dummy auth
- Secret sync dry-run

## Metrics

- Token refresh cycles (with duration_seconds histogram)
- Secret copy operations (with duration_seconds histogram)
- Error rates
- Concurrent operations

## Error Handling

- Log clearly and block admission for ECR-related failures:
    - Missing or invalid ECR pull secret
    - Failed secret validation
    - Missing imagePullSecret reference
- Retry with exponential backoff on:
    - Kubernetes API operations
    - ECR token pull
    - Secret patch failures
- Track error rates
- Alert on persistent failures
- Return clear admission denial messages for ECR validation failures

## Race Conditions

- Use Kubernetes API create/update operations for idempotency
- Track namespace operations
- Handle concurrent requests
- Monitor for conflicts

## RBAC Requirements

The webhook requires the following permissions:
- Namespaced secrets (create, update, get)
- Namespaced serviceaccounts (get, patch)
- Namespaces (get, list)
- ConfigMaps in gtoken namespace (create, update, get)

## Security Considerations

1. ECR secret permissions should be limited to pull-only unless push is required
2. Webhook RBAC should be scoped to minimum required permissions
3. Token refresh includes safety buffer to prevent expiration
4. Failed operations don't block pod admission
5. All operations are idempotent

## Monitoring

The webhook provides comprehensive monitoring through:
1. Prometheus metrics for all ECR operations
2. Health check endpoints (/healthz, /selftest)
3. Structured logging for operational visibility
4. Error tracking and alerting capabilities

## Disaster Recovery

1. ConfigMap stores token state for recovery
2. Failed operations are retried with backoff
3. Pod admission continues even if secret sync fails 