# Google OAuth API Documentation

This document describes the Google OAuth integration for NomNom Radar.

## Overview

The Google OAuth integration supports two authentication flows:
1. **ID Token Flow** (Recommended) - Direct verification of Google ID tokens
2. **Authorization Code Flow** (Future) - Exchange authorization codes for tokens

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
  "oauth_url": "https://accounts.google.com/oauth/authorize?client_id=...&redirect_uri=...&scope=...&response_type=code",
  "redirect_url": "/oauth/google?redirect=true",
  "note": "Use redirect_url for direct redirect, or oauth_url for frontend implementation"
}
```

**Redirect Response (when `redirect=true`):**
- HTTP 302 redirect to Google OAuth page

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
  "id_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

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

## Configuration

### Required Environment Variables

Add the following to your `config.yaml`:

```yaml
googleOAuth:
  clientId: "your_google_client_id_here"
  clientSecret: "your_google_client_secret_here"
  redirectUri: "http://localhost:4433/oauth/google/callback"
  scopes: "openid email profile"
```

### Google Console Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing one
3. Enable Google+ API
4. Go to "Credentials" → "Create Credentials" → "OAuth 2.0 Client IDs"
5. Configure authorized redirect URIs:
   - `http://localhost:4433/oauth/google/callback` (for development)
   - `https://yourdomain.com/oauth/google/callback` (for production)

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
function handleCredentialResponse(response) {
    fetch('/oauth/google/callback', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id_token: response.credential })
    })
    .then(response => response.json())
    .then(data => {
        // Store tokens and redirect
        localStorage.setItem('access_token', data.access_token);
        localStorage.setItem('refresh_token', data.refresh_token);
        window.location.href = '/dashboard';
    });
}
</script>
```

### Method 2: Manual OAuth Flow

```javascript
// Step 1: Get OAuth URL
const response = await fetch('/oauth/google');
const data = await response.json();

// Step 2: Redirect to Google
window.location.href = data.oauth_url;

// Step 3: Handle callback (you'll need to implement this)
// Google will redirect back to your redirect URI with an authorization code
```

## Security Considerations

1. **ID Token Verification**: The backend verifies:
   - Token signature (issuer validation)
   - Audience (client ID validation)
   - Expiration time
   - Email verification status

2. **HTTPS**: Always use HTTPS in production

3. **Client ID Validation**: Ensure the client ID matches your Google Console configuration

4. **Token Storage**: Store tokens securely (httpOnly cookies recommended)

## Error Handling

Common error scenarios:

1. **Invalid ID Token**: Token format is incorrect or expired
2. **Invalid Audience**: Client ID doesn't match
3. **Email Not Verified**: User's email is not verified with Google
4. **Token Expired**: ID token has expired

## Testing

Use the provided example page (`examples/google-login.html`) to test the integration:

1. Update the `YOUR_GOOGLE_CLIENT_ID` in the HTML file
2. Open the page in a browser
3. Test different authentication methods

## Future Enhancements

1. **Authorization Code Flow**: Implement full OAuth 2.0 authorization code flow
2. **Refresh Token Handling**: Automatic token refresh
3. **Google People API**: Fetch additional user profile information
4. **Multiple OAuth Providers**: Support for GitHub, Facebook, etc.
