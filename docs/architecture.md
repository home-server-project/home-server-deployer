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

Alpha 0 uses the portable Podman 5.8.2-6.1.x common denominator even when newer capabilities exist.

Install:

1. resolve and validate catalog input;
2. resolve directories as `approved-root ID + relative path`;
3. render trusted Quadlet templates;
4. create a plan containing hashes, affected paths/resources, and a digest;
5. install individual Quadlets normally with Podman's `reload-systemd=false` behavior;
6. perform one direct systemd daemon reload;
7. resolve generated unit names from `podman quadlet list`;
8. start and verify only units belonging to the managed instance;
9. read the live installed Quadlet back from Podman and record its baseline SHA-256;
10. atomically persist instance history and generated documentation.

Update deliberately does not use native `replace=true`:

1. immediately before execution, re-check administrator drift;
2. snapshot every current managed Quadlet from the live Podman source;
3. stop current managed units;
4. remove current managed Quadlets with systemd reload deferred;
5. install the new Quadlets normally with systemd reload deferred;
6. if installation fails, remove only sources installed by this transaction and restore the snapshots before reload;
7. perform one direct systemd daemon reload;
8. resolve and restart/start the new units and verify systemd reports them active;
9. if reload/start/verification fails, stop the attempted new units, remove the attempted new source set, restore the snapshotted previous source, reload once, and restart/verify the previous units;
10. after successful verification, persist the new live-source hashes/state/documentation.

This common path is intentional. Podman 5.7.0 through 5.8.5 are affected by GHSA-fx76-2j3w-2mx6 / CVE-2026-19730, in which native Quadlet replace can retain stale trailing bytes when a new source is shorter. Podman 5.8.6 fixes the bug and Podman 6.x is unaffected, but Alpha 0 keeps identical safe remove/install semantics across the supported range. A future backend capability such as `safeNativeReplace` may optimize newer versions without changing the application/catalog model.

## Podman compatibility backend

The catalog and engine do not encode Podman private storage details.

`internal/podman.Backend` is the stable engine-facing contract. `HTTPClient` currently implements it using the Libpod REST API.

Capability discovery combines:

- `/version` engine and Libpod API versions;
- a non-mutating Quadlet-list endpoint probe;
- `/libpod/info` host properties;
- an isolated version capability profile for API features that cannot be safely probed through a mutating request.

Capability introduction and support policy are separate concepts:

- the Quadlet REST API capability family is modeled as available from Podman 5.8.0;
- Home Server Deployer's supported floor is Podman 5.8.2;
- Podman 5.8.2, 5.8.4, and 6.1.x are current Alpha 0 compatibility targets;
- Podman 6.1.x native application installation is reported as available, while Alpha 0 still uses individual Quadlets.

A later backend may opt into useful newer-Podman native application operations without changing the catalog/application model.

No code reads or modifies Podman's old `.app` files or newer application subdirectories directly.

## Persistence

Default Agent-visible locations:

- state: `/var/lib/home-server-deployer`;
- generated docs: `/var/lib/home-server-deployer/docs`;
- approved application-data host root: `/var/lib/home-server-apps` mounted into Agent as `/approved/application-data`.

The documentation root is configurable in `config.json`. Approved roots carry separate host and Agent paths so the catalog never receives arbitrary absolute-path authority.

State files use write -> fsync -> rename semantics. Every resource stores a last-known live source snapshot and SHA-256 baseline for drift detection/recovery documentation. The snapshot is metadata; the installed native Quadlet remains authoritative.
