# API Conventions

This document is the source of truth for HTTP response envelopes shared by the NomNom-Radar API. Focused endpoint references document the value carried in `data` or the endpoint-specific error behavior rather than repeating these envelopes.

## Success Envelope

Successful responses place the endpoint result in `data` and include the request correlation ID in `meta`:

```json
{
  "data": {},
  "meta": {
    "request_id": "request-correlation-id"
  }
}
```

`data` may be an object, array, or endpoint-specific scalar value. The HTTP status code communicates whether the operation returned, created, or otherwise completed a resource.

## Error Envelope

Error responses use a stable machine-readable code and a client-safe message:

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Invalid request",
    "details": "field-specific client-safe context"
  },
  "meta": {
    "request_id": "request-correlation-id"
  }
}
```

`error.details` is optional. It is omitted for server errors and authentication or authorization failures, including HTTP `401` and `403`. Responses never expose stack traces, SQL details, internal paths, or raw internal errors.

## Request IDs

Clients should retain `meta.request_id` when reporting failures or unexpected behavior. The same correlation ID is used by the server's request lifecycle logging.

Clients may send an `X-Request-Id` header. The API server and the geo worker keep it only when it is 1-128 characters of ASCII letters, digits, `.`, `_`, or `-`; otherwise they replace it with a new UUID without returning an error. The chosen ID is echoed in the `X-Request-Id` response header. API responses also carry it in `meta.request_id`. Geo worker `/push` responses have no body, so they carry it only in the header; worker error-envelope responses, such as recovered panics and unmatched routes, also include `meta.request_id`.
