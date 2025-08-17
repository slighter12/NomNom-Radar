# Google OAuth API Documentation

This document describes the Google OAuth integration for NomNom Radar.

## Overview

The Google OAuth integration supports two authentication flows:
1. **ID Token Flow** (Recommended) - Direct verification of Google ID tokens
2. **Authorization Code Flow** (Future) - Exchange authorization codes for tokens

## Security Features

- **CSRF Protection**: All OAuth flows include a cryptographically secure `state` parameter to prevent Cross-Site Request Forgery attacks
- **State Expiration**: State parameters expire after 10 minutes and are single-use only
- **Secure State Storage**: State parameters are stored in memory with automatic cleanup

## API Endpoints

### 1. Initiate Google OAuth Flow

**Endpoint:** `GET /oauth/google`

**Description:** Generates a Google OAuth authorization URL or redirects to Google.

**Query Parameters:**
- `redirect` (optional): Set to `true` to redirect directly to Google

**Response Examples:**

**JSON Response (default):**
```json
{
  "message": "Google OAuth URL generated successfully",
  "oauth_url": "https://accounts.google.com/oauth/authorize?client_id=...&redirect_uri=...&scope=...&response_type=code&state=...",
  "state": "generated_state_parameter",
  "redirect_url": "/oauth/google?redirect=true",
  "note": "Use redirect_url for direct redirect, or oauth_url for frontend implementation. Store the state parameter to verify the callback."
}
```

**Redirect Response (when `redirect=true`):**
- HTTP 302 redirect to Google OAuth page

**Security Notes:**
- The `state` parameter is automatically generated and included in the OAuth URL
- Store this `state` parameter securely to validate the callback
- The state expires after 10 minutes and can only be used once

**Usage:**
```bash
# Get OAuth URL for frontend
curl http://localhost:4433/oauth/google

# Redirect directly to Google
curl http://localhost:4433/oauth/google?redirect=true
```

### 2. Google OAuth Callback

**Endpoint:** `POST /oauth/google/callback`

**Description:** Handles Google OAuth callback and verifies ID tokens.

**Request Body:**
```json
{
  "id_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "state": "stored_state_parameter"
}
```

**Query Parameters (Alternative):**
- `id_token`: Google ID token
- `state`: State parameter for CSRF protection

**Response:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "refresh_token_here",
  "user": {
    "id": "uuid-here",
    "name": "John Doe",
    "email": "john@example.com",
    "user_profile": {},
    "merchant_profile": null
  }
}
```

**Error Response:**
```json
{
  "error": "Invalid ID token"
}
```

**CSRF Protection:**
- The `state` parameter must match the one generated during the initial OAuth request
- Invalid or expired state parameters will return a `400 Bad Request` error
- State parameters are automatically validated and cleaned up after use

## Frontend Integration

### Method 1: Google Sign-In Button (Recommended)

```html
<script src="https://accounts.google.com/gsi/client" async defer></script>

<div id="g_id_onload"
     data-client_id="YOUR_GOOGLE_CLIENT_ID"
     data-callback="handleCredentialResponse"
     data-auto_prompt="false">
</div>
<div class="g_id_signin"
     data-type="standard"
     data-size="large"
     data-theme="outline"
     data-text="sign_in_with"
     data-shape="rectangular"
     data-logo_alignment="left">
</div>

<script>
async function handleCredentialResponse(response) {
    // Step 1: Get OAuth URL with state parameter
    const oauthResponse = await fetch('/oauth/google');
    const oauthData = await oauthResponse.json();
    
    // Store the state parameter
    const state = oauthData.state;
    
    // Step 2: Send ID token with state for verification
    const callbackResponse = await fetch('/oauth/google/callback', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
            id_token: response.credential,
            state: state
        })
    });
    
    const data = await callbackResponse.json();
    
    // Store tokens and redirect
    localStorage.setItem('access_token', data.access_token);
    localStorage.setItem('refresh_token', data.refresh_token);
    window.location.href = '/dashboard';
}
</script>
```

### Method 2: Manual OAuth Flow

```javascript
// Step 1: Get OAuth URL with state parameter
const response = await fetch('/oauth/google');
const data = await response.json();

// Store the state parameter securely
const state = data.state;

// Step 2: Redirect to Google
window.location.href = data.oauth_url;

// Step 3: Handle callback with state validation
// Google will redirect back to your redirect URI with an authorization code and state
// You'll need to implement the callback handler to validate the state parameter
```

## Security Considerations

1. **State Parameter Validation**: Always validate the state parameter in callbacks
2. **ID Token Verification**: The backend verifies:
   - Token signature (issuer validation)
   - Audience (client ID validation)
   - Expiration time
   - Email verification status
3. **HTTPS**: Always use HTTPS in production
4. **Client ID Validation**: Ensure the client ID matches your Google Console configuration
5. **Token Storage**: Store tokens securely (httpOnly cookies recommended)
6. **State Storage**: Store state parameters securely on the client side

## Error Handling

Common error scenarios:

1. **Invalid State Parameter**: State is missing, expired, or already used
2. **Invalid ID Token**: Token format is incorrect or expired
3. **Invalid Audience**: Client ID doesn't match
4. **Email Not Verified**: User's email is not verified with Google
5. **Token Expired**: ID token has expired

## Testing

Use the provided example page (`examples/google-login.html`) to test the integration:

1. Update the `YOUR_GOOGLE_CLIENT_ID` in the HTML file
2. Open the page in a browser
3. Test different authentication methods
4. Verify state parameter validation

## Future Enhancements

1. **Authorization Code Flow**: Implement full OAuth 2.0 authorization code flow
2. **Refresh Token Handling**: Automatic token refresh
3. **Google People API**: Fetch additional user profile information
4. **Multiple OAuth Providers**: Support for GitHub, Facebook, etc.
5. **Enhanced State Management**: Database-backed state storage for distributed systems
