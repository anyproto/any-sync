# Secure Service

The secure service owns peer identity verification for transport handshakes.
Federated admission can optionally add JWT-based network admission checks to that handshake.

## Admission JWT configuration

Admission is disabled by default. When enabled, inbound handshakes with an admission token are verified against the configured issuer, audience, JWKS, network claim, and Anytype identity claim.

```yaml
secureService:
  admission:
    enabled: true
    required: true
    issuer: https://issuer.example
    audience: any-sync
    jwksUrl: https://issuer.example/.well-known/jwks.json
    requiredClaims:
      group: docs-users
    identityClaim: anytype_identity
    networkClaim: network_id
    subjectClaim: sub
    clockSkewSec: 60
```

`enabled: true` verifies a token when a peer sends one. The stock JWT verifier binds tokens to signed Anytype peer credentials, so optional admission also requires a signed peer identity when a token is present. `required: true` rejects peers that do not send a token and forces signed peer credentials for inbound admission checks.

When admission is enabled and no custom verifier is injected, `issuer`, `audience`, and `jwksUrl` are required. JWKS keys are fetched during secure-service initialization and reused until restart.

The token's network claim must match the node configuration network ID. The token's identity claim must match the Anytype account identity presented in the signed peer credentials. Tokens must include a valid `exp` claim; `nbf` and `iat` are enforced when present. `requiredClaims` entries must also match when configured.

Default claim names are:

- `identityClaim`: `anytype_identity`
- `networkClaim`: `network_id`
- `subjectClaim`: `sub`

Default clock skew is:

- `clockSkewSec`: `60`

Outbound admission tokens can be supplied per dial context with `CtxWithOutboundAdmissionToken` or by registering an `AdmissionTokenProvider` component. Secure-service logs record admission outcomes but do not log bearer tokens, verifier error strings, rejection reasons, or claims.

This configuration validates bearer JWTs from the configured JWKS. It does not perform OIDC discovery or SAML assertion validation.
