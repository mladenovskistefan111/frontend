# Known limitations

- `Secure` + `SameSite=Strict` cookies break carts over plain HTTP outside `localhost`.
- Health is static; readiness doesn't reflect backend availability.
- Debug-level JSON logging is unconditional.
- `templates/assistant.html` and the header's assistant link exist, but nothing sets the `assistant_enabled` flag and no assistant routes are registered — the AI shopping assistant is dormant.
- Per-request gRPC calls have no retries; the ad call has a 100 ms deadline, the rest use the request context.
