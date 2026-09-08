# Alpha 0 architecture

## Design invariant

Native Quadlet files are authoritative. Deployer state is ownership/baseline/history/recovery metadata; it is not a replacement desired-state database.

A managed application should continue to run if both Deployer containers disappear.

## Processes

### Web

`deployer-web` is the browser-facing component.

It:

- runs as fixed unprivileged UID/GID 65532;
- receives no Podman or systemd socket;
- receives no Deployer state directory;
- receives no application-data root;
- communicates with Agent only over `/run/home-server-deployer/agent.sock` in a private shared named volume;
- uses same-origin forms with a cryptographically random CSRF token;
- provides only a primitive Alpha 0 test interface.

### Agent

`deployer-agent` is intentionally privileged because rootful `/run/podman/podman.sock` is root-equivalent.

It:

- publishes no network port and runs with `Network=none`;
- talks to Podman through the rootful Unix socket;
- talks directly to systemd through `/run/systemd/private` using `go-systemd`, avoiding a generic system D-Bus mount;
- owns the only write access to Deployer state/docs and configured approved application roots;
- has no generic shell endpoint, file-manager endpoint, Podman proxy, sysctl endpoint, or SELinux administration endpoint.

The supplied Agent Quadlet uses `SecurityLabelDisable=true` because a container that controls the host rootful Podman socket cannot meaningfully be claimed to remain confined by its normal container SELinux domain. Security is therefore primarily the narrow API plus narrow mount/socket set.

## Internal API

The Agent listens only on a Unix domain socket. The current `/v1` surface contains typed operations for:

- capabilities;
- catalog;
- Quadlet discovery;
- managed instances/status;
- create plan;
- execute plan;
- start/stop/restart of a managed instance;
- logs for runtime containers mapped to managed units;
- generated documentation;
- backup-contract inspection.

There is deliberately no endpoint accepting an arbitrary shell command, arbitrary host path, arbitrary systemd unit, arbitrary Podman URL, or arbitrary SELinux label.

## Transaction model

Alpha 0 uses the portable 5.8.4-6.1.x common denominator even when newer capabilities exist:

1. resolve and validate catalog input;
2. resolve directories as `approved-root ID + relative path`;
3. render trusted Quadlet templates;
4. create a plan containing hashes, affected paths/resources, and a digest;
5. immediately before execution, re-check administrator drift;
6. install/replace individual Quadlets with Podman's `reload-systemd=false` behavior;
7. perform one direct systemd daemon reload;
8. resolve generated unit names from `podman quadlet list`;
9. start/restart only units belonging to the managed instance;
10. read the live installed Quadlet back from Podman and record its baseline SHA-256;
11. atomically persist instance history and generated documentation.

On a failed update, the previously read live Quadlet source is reinstalled and systemd is reloaded/restarted. The rollback does not depend on the catalog template that caused the failed update.

## Podman compatibility backend

The catalog and engine do not encode Podman private storage details.

`internal/podman.Backend` is the stable engine-facing contract. `HTTPClient` currently implements it using the Libpod REST API.

Capability discovery combines:

- `/version` engine and Libpod API versions;
- a non-mutating Quadlet-list endpoint probe;
- `/libpod/info` host properties;
- an isolated version capability profile for API features that cannot be safely probed through a mutating request.

For Podman 6.1.x, native application installation is reported as available. Alpha 0 still uses individual Quadlets. A later backend may opt into native application operations without changing the catalog/application model.

No code reads or modifies Podman's old `.app` files or newer application subdirectories directly.

## Persistence

Default Agent-visible locations:

- state: `/var/lib/home-server-deployer`;
- generated docs: `/var/lib/home-server-deployer/docs`;
- approved application-data host root: `/var/lib/home-server-apps` mounted into Agent as `/approved/application-data`.

The documentation root is configurable in `config.json`. Approved roots carry separate host and Agent paths so the catalog never receives arbitrary absolute-path authority.

State files use write -> fsync -> rename semantics. Every resource stores a last-known live source snapshot and SHA-256 baseline for drift detection/recovery documentation. The snapshot is metadata; the installed native Quadlet remains authoritative.
