# Google OAuth API

This document is the source of truth for the Google mobile ID-token, provider-linking, and merchant-onboarding client contract. Shared HTTP envelopes are documented in `docs/reference/api-conventions.md`; the JSON examples below show the endpoint-specific value inside `data`.

## Contract Overview

The mobile client completes Google Sign-In and sends the resulting ID token to the backend. The backend verifies the token with Google's official verifier and uses `(provider, provider_user_id)` as the provider identity key.

Email matching does not silently link accounts. When a verified Google email matches an existing local account without that provider identity, the backend returns `linking_required`; the client must re-authenticate the existing account before linking.

The backend requires only the Google client ID for ID-token audience validation:

```yaml
googleOAuth:
  clientId: "YOUR_GOOGLE_CLIENT_ID"
```

OAuth client secrets, backend redirect URIs, and authorization-code exchange are not part of this mobile ID-token flow.

## Auth Result

Authentication endpoints return one of these values inside the success envelope's `data` field.

Authenticated:

```json
{
  "status": "authenticated",
  "access_token": "jwt-access-token",
  "refresh_token": "jwt-refresh-token",
  "user": {
    "id": "uuid",
    "email": "user@example.com"
  }
}
```

Merchant onboarding required:

```json
{
  "status": "onboarding_required",
  "onboarding_token": "short-lived-jwt",
  "requested_role": "merchant",
  "required_fields": ["store_name"]
}
```

Existing-account re-authentication required:

```json
{
  "status": "linking_required",
  "linking_token": "short-lived-jwt"
}
```

Clients must treat roles in JWT claims as a set. Role order does not define a primary role.

## Client Flow

1. Complete Google Sign-In in the mobile client and obtain an ID token.
2. Send the ID token and requested role to `POST /oauth/google/callback`.
3. On `authenticated`, store and use the returned session tokens.
4. On `onboarding_required`, collect the required merchant fields and call `POST /auth/onboarding/merchant`.
5. On `linking_required`, re-authenticate the existing email/password account and call `POST /auth/link-provider`.

Business-license verification is a later authenticated merchant action. It is not part of provider linking or token-based merchant onboarding.

## Endpoints

### POST /oauth/google/callback

Verifies a Google ID token and runs the unified authentication flow.

User request:

```json
{
  "id_token": "google-id-token",
  "requested_role": "user"
}
```

Merchant request may include the currently required profile field:

```json
{
  "id_token": "google-id-token",
  "requested_role": "merchant",
  "store_name": "NomNom Bento"
}
```

`requested_role` accepts `user` or `merchant`. For backward compatibility, `state=user` and `state=merchant` are still accepted as a deprecated fallback; clients must not introduce new uses of `state`.

The result is `authenticated`, `onboarding_required`, or `linking_required` as described above.

### POST /auth/link-provider

Links the verified provider identity to an existing email/password account after re-authentication.

```json
{
  "linking_token": "short-lived-jwt",
  "password": "current-password"
}
```

The linking token alone cannot attach the provider. The current account password is required, and expired, invalid, or reused linking tokens are rejected. The result is `authenticated`, or `onboarding_required` when the original merchant intent still lacks required profile data.

### POST /auth/onboarding/merchant

Completes merchant onboarding after an `onboarding_required` result.

```json
{
  "onboarding_token": "short-lived-jwt",
  "store_name": "NomNom Bento"
}
```

Success returns an `authenticated` result. Completing onboarding more than once returns HTTP `409`, and an onboarding token cannot be reused to mint another session after profile creation.

### POST /api/v1/merchant/verification

Submits a business license after merchant account creation. This endpoint requires an authenticated merchant account.

```json
{
  "business_license": "A123456789"
}
```

Success returns this value inside `data`:

```json
{
  "status": "verified"
}
```

An active license already owned by another merchant returns HTTP `409` with code `BUSINESS_LICENSE_ALREADY_EXISTS`. Accepted submissions are currently verified immediately. A verified merchant cannot self-service change to a different business license; that requires an operational support path.

## Security Rules

- Google token signature, expiration, issuer, and audience must be verified by the backend.
- The Google email must be verified; required email and name claims must be present. Picture and locale claims are optional.
- Registration with an email already attached to an existing account returns HTTP `409`; registration is not an account-linking operation.
- Verified email matching identifies a possible local account but never authorizes linking without re-authentication.
- Short-lived linking and onboarding tokens are purpose-specific and cannot replace normal authentication.
- Future providers with relay or hidden email addresses require an explicit "I already have an account" linking path rather than relying only on email matching.
