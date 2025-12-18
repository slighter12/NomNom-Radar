# Google OAuth Architecture

## Overview

This document describes the Google OAuth architecture for the NomNom-Radar application. The architecture follows the **ID Token Verification** pattern, which is the recommended approach for mobile applications using Google Sign-In.

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
    const response = await fetch('/api/oauth/google/callback', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id_token: idToken })
    });

    // Handle response...
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
        return errors.WithStack(err)
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

## API Endpoints

### POST /api/oauth/google/callback

- **Purpose**: Verify Google ID token and authenticate user
- **Input**: `{ "id_token": "google_id_token" }`
- **Output**: User authentication result with session tokens

### GET /api/oauth/google (Deprecated)

- **Status**: Returns `NOT_IMPLEMENTED`
- **Reason**: OAuth URL generation moved to frontend

## Migration Guide

### For Frontend Developers

1. Install `@react-native-google-signin/google-signin`
2. Configure with your `webClientId`
3. Use `GoogleSignin.signIn()` to get ID token
4. Send ID token to `/api/oauth/google/callback`

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
- Test user creation/authentication flow

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
