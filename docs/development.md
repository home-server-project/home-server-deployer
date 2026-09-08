# Development notes

## Core workflow

Repository work follows **Propose -> Greenlight -> Execute**.

Production Home Server access is reference/read-only. Destructive application, SELinux, failure-injection, rollback, and remove testing belongs only in the dedicated VM.

## Local checks

```sh
make check
```

`make check` runs formatting verification, `go vet`, and Go tests. The full dependency graph requires normal network access to download Go modules on a fresh checkout.

## Engine interfaces

Keep privileged host concerns behind narrow interfaces:

- `podman.Backend`;
- `systemd.Manager`;
- `security.Backend`;
- future `backup.Provider` implementations.

Do not leak Podman-version-specific fields or private Podman storage layout into the catalog schema.

Podman API capability introduction and Home Server Deployer support policy are separate. The Quadlet REST capability family begins at Podman 5.8.0; the current Deployer-supported floor is 5.8.2.

Alpha 0 must not use native Quadlet `replace=true`. Updates use stop -> remove with reload deferred -> normal install with reload deferred -> one reload -> start/restart/verify, with source-snapshot rollback. This avoids GHSA-fx76-2j3w-2mx6 / CVE-2026-19730 on affected Podman 5.8.x while keeping one transaction model for 6.1.x.

## Catalog rules

Catalog YAML is decoded with unknown fields rejected.

User input is typed and validated. It must never become a new Quadlet directive through embedded newline/control characters.

Templates are trusted repository content. The browser can select typed values but cannot supply template source.

Application definitions describe security, backup, update, and health intent rather than host-specific command sequences.

## State rules

Native installed Quadlet content is authoritative.

Deployer state may retain source snapshots for baseline comparison, rollback and documentation, but code must always re-read the live Quadlet before destructive mutation.

Application data is not deleted by normal application removal.
