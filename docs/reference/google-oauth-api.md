# Google OAuth API

## Overview

This document describes the Google OAuth API contract for NomNom-Radar. The backend follows the **ID Token Verification** pattern and returns a unified `AuthResult` envelope shared with email registration and login.

Provider identity must be keyed by `(provider, provider_user_id)`. Email matching is only used to detect a possible existing local account that requires re-authentication before linking.

Future Sign in with Apple support needs an explicit account-linking fallback: users can choose Hide My Email, causing Apple to return an `@privaterelay.appleid.com` relay address instead of the user's real email. In that case, email matching may not find the user's existing email/password account, so the client must offer an "I already have an account" path that signs into the existing account before linking the Apple provider.

## Architecture Design

The architecture is designed with clear separation of responsibilities:

- **Frontend**: Handles the entire OAuth flow using Google Sign-In SDK
- **Backend**: Only verifies ID tokens received from the frontend
- **Configuration**: Backend only requires `ClientID` for token verification
- **Security**: Uses Google's official library for automatic security validation

## How It Works

### 1. Frontend OAuth Flow (React Native)

```typescript
// Using @react-native-google-signin/google-signin
import { GoogleSignin } from '@react-native-google-signin/google-signin';

// Configure the SDK (done once during app initialization)
GoogleSignin.configure({
  webClientId: 'YOUR_WEB_CLIENT_ID', // From GoogleService-Info.plist
  offlineAccess: false,
});

// Handle sign-in
const signIn = async () => {
  try {
    const { idToken } = await GoogleSignin.signIn();

    // Send ID token to backend for verification
    const response = await fetch('/oauth/google/callback', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        id_token: idToken,
        requested_role: 'user',
      }),
    });

    const result = await response.json();

    if (result.status === 'authenticated') {
      // use access_token / refresh_token
    }

    if (result.status === 'onboarding_required') {
      // store onboarding_token and continue merchant onboarding
    }
  } catch (error) {
    // Handle error...
  }
};
```

### 2. Backend Token Verification

```go
// Backend receives ID token and verifies it using Google's official library
func (h *UserHandler) GoogleCallback(c echo.Context) error {
    // Extract ID token from request
    idToken := c.FormValue("id_token")

    // Verify the token using Google's official idtoken.Validate function
    oauthUser, err := h.googleAuthService.VerifyIDToken(ctx, idToken)
    if err != nil {
        return err
    }

    // Process the verified user information
    // Create/update user account, generate session tokens, etc.
}
```

## Backend Implementation

### OAuth Service

```go
type OAuthService struct {
    clientID string
    logger   *slog.Logger
}

func (s *OAuthService) VerifyIDToken(ctx context.Context, idToken string) (*service.OAuthUser, error) {
    // Use Google's official library for token validation
    payload, err := idtoken.Validate(ctx, idToken, s.clientID)
    if err != nil {
        s.logger.Error("Google token validation failed", "error", err)
        return nil, fmt.Errorf("invalid id token: %w", err)
    }

    // The library automatically handles:
    // - JWT signature verification using Google's public keys
    // - Token expiration checking
    // - Issuer validation (ensures token is from Google)
    // - Audience validation (ensures token is for your app)

    claims := payload.Claims

    // Check email verification
    if emailVerified, ok := claims["email_verified"].(bool); !ok || !emailVerified {
        return nil, fmt.Errorf("email not verified")
    }

    // Extract user information from verified claims
    oauthUser := &service.OAuthUser{
        ID:            payload.Subject,
        Email:         claims["email"].(string),
        Name:          claims["name"].(string),
        Provider:      entity.ProviderTypeGoogle,
        AvatarURL:     claims["picture"].(string),
        EmailVerified: true,
        Locale:        claims["locale"].(string),
        ExtraData:     claims,
    }

    return oauthUser, nil
}
```

## Configuration

### Required Configuration

```yaml
googleOAuth:
  clientId: "YOUR_GOOGLE_CLIENT_ID"
```

### What You Need

1. **GoogleService-Info.plist** - Place in your iOS project
2. **google-services.json** - Place in your Android project
3. **Web Client ID** - Use this in your React Native configuration

### What You DON'T Need

- OAuth secrets - Not needed for ID token verification
- Redirect URIs - Not needed for mobile OAuth flows
- Scopes - Handled by the Google Sign-In SDK

## Security Benefits

1. **No Secret Exposure**: Backend doesn't handle OAuth secrets
2. **Official Library**: Uses Google's official `google.golang.org/api/idtoken` package
3. **Automatic Security**: Library handles all security checks automatically
4. **Audience Validation**: Ensures tokens were issued for your app
5. **Expiration Checking**: Automatic token expiration validation
6. **Email Verification**: Ensures user's email is verified by Google

## AuthResult Envelope

All auth-related endpoints below return the same envelope:

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

or:

```json
{
  "status": "onboarding_required",
  "onboarding_token": "short-lived-jwt",
  "requested_role": "merchant",
  "required_fields": ["store_name"]
}
```

or:

```json
{
  "status": "linking_required",
  "linking_token": "short-lived-jwt"
}
```

Registration is not an account-linking flow. If an email is already present on an existing account, `/auth/register/user` and `/auth/register/merchant` must return `409 conflict` instead of attaching a new login method.

OAuth login by email match is not an account-linking flow either. When a verified OAuth email matches an existing local account that does not already have the provider identity attached, the backend returns `status=linking_required`. The client must re-authenticate the existing account and then call `/auth/link-provider` with the short-lived linking token.

Role order in JWT claims is not a primary-role contract. Clients must treat roles as a set; if the product needs a primary role later, add an explicit field instead of inferring it from array position.

## Merchant OAuth Workflow

Merchant OAuth has two separate completion steps:

1. OAuth login or provider linking verifies the identity provider account.
2. If the requested merchant role still lacks required merchant profile data, the auth response returns `status=onboarding_required`.
3. The client calls `/auth/onboarding/merchant` with the `onboarding_token` and required profile fields such as `store_name`.
4. After onboarding completes, the client receives an authenticated merchant session.
5. Business license verification is a later authenticated merchant action through `/api/v1/merchant/verification`; it is not part of the OAuth linking or onboarding token flow.

## API Endpoints

### POST /oauth/google/callback

- **Purpose**: Verify Google ID token and authenticate user
- **Input**:

```json
{
  "id_token": "google_id_token",
  "requested_role": "user"
}
```

- **Merchant input**:

```json
{
  "id_token": "google_id_token",
  "requested_role": "merchant",
  "store_name": "NomNom Bento"
}
```

- **Backward compatibility**:
  - `state` is still accepted as a deprecated fallback.
  - Only `state=user` and `state=merchant` are supported.
- **Output**:
  - `status=authenticated` when the account is fully ready
  - `status=onboarding_required` when merchant profile data is still missing
  - `status=linking_required` when a verified OAuth email matches an existing local account and re-authentication is required before linking

### POST /auth/link-provider

- **Purpose**: Link an OAuth provider to an existing account after the user re-authenticates.
- **Input**:

```json
{
  "linking_token": "short-lived-jwt",
  "password": "current-password"
}
```

- **Output**:
  - `status=authenticated` with `access_token`, `refresh_token`, and `user`
  - `status=onboarding_required` when the linking token carries merchant intent and required merchant onboarding fields are still missing
- **Security behavior**:
  - The linking token alone is not enough to attach the provider.
  - The existing account must be re-authenticated.
  - Reused or expired linking tokens must be rejected.

### POST /auth/onboarding/merchant

- **Purpose**: Complete merchant onboarding after an OAuth sign-in returned `onboarding_required`
- **Input**:

```json
{
  "onboarding_token": "short-lived-jwt",
  "store_name": "NomNom Bento"
}
```

- **Output**:
  - `status=authenticated` with `access_token`, `refresh_token`, and `user`
- **Replay behavior**:
  - If onboarding has already been completed for that account, the endpoint returns `409 conflict`
  - The same onboarding token must not be reused to mint additional sessions after merchant profile creation

### POST /api/v1/merchant/verification

- **Purpose**: Submit a merchant business license after account creation.
- **Auth**: Requires an authenticated merchant account.
- **Input**:

```json
{
  "business_license": "A123456789"
}
```

- **Output**:
  - `status=verified` when the license is accepted
  - `409 BUSINESS_LICENSE_ALREADY_EXISTS` when another active merchant already uses the same license
- **Verification behavior**:
  - The current flow auto-verifies accepted submissions immediately and stores the merchant as `verified`
  - After a merchant is `verified`, the merchant cannot self-service change `business_license`; changes require an operational support path

### GET /api/oauth/google (Deprecated)

- **Status**: Returns `NOT_IMPLEMENTED`
- **Reason**: OAuth URL generation moved to frontend

## Migration Guide

### For Frontend Developers

1. Install `@react-native-google-signin/google-signin`
2. Configure with your `webClientId`
3. Use `GoogleSignin.signIn()` to get ID token
4. Send ID token to `/oauth/google/callback`
5. If response is `onboarding_required`, call `/auth/onboarding/merchant`

### For Backend Developers

1. Remove OAuth URL building logic
2. Focus on ID token verification using `idtoken.Validate`
3. Update configuration to only include `ClientID`
4. Remove state management and CSRF protection for OAuth

## Dependencies

### Go Dependencies

```go
import "google.golang.org/api/idtoken"
```

The `idtoken.Validate` function automatically:

- Fetches and caches Google's public keys
- Verifies JWT signatures
- Checks token expiration
- Validates issuer and audience claims

## Testing

### Frontend Testing

- Test Google Sign-In SDK integration
- Verify ID token generation
- Test error handling for failed sign-ins

### Backend Testing

- Test ID token verification with valid tokens
- Test rejection of invalid/expired tokens
- Test audience validation
- Test unified auth flow for:
  - same-email account linking
  - merchant onboarding_required responses
  - onboarding replay returning conflict

## Troubleshooting

### Common Issues

1. **"Invalid audience" error**: Check that `ClientID` matches your Google project
2. **"Token expired" error**: Ensure tokens are sent promptly after generation
3. **"Email not verified" error**: User must verify email with Google first

### Debug Steps

1. Verify `ClientID` in configuration
2. Check Google Cloud Console project settings
3. Ensure Google Sign-In API is enabled
4. Verify OAuth consent screen configuration

## References

- [Google Sign-In for iOS](https://developers.google.com/identity/sign-in/ios)
- [Google Sign-In for Android](https://developers.google.com/identity/sign-in/android)
- [Google Sign-In for Web](https://developers.google.com/identity/sign-in/web)
- [ID Token Verification](https://developers.google.com/identity/sign-in/ios/backend-auth)
- [Google Go ID Token Package](https://pkg.go.dev/google.golang.org/api/idtoken)
