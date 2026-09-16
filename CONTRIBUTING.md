# Contributing

## Development

```bash
go test ./...
go build ./cmd/v46lift
```

Keep translation engines behind the backend interface. Application-specific behavior does not belong in the core runtime.

## Design rules

1. Never inject into the target process.
2. Never require the target process to run elevated.
3. Keep network privilege in the runtime/service boundary.
4. Preserve TCP/UDP payloads unchanged.
5. Prefer explicit, deterministic mappings over protocol sniffing.
