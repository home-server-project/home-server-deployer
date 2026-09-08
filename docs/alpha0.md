# Alpha 0 scope and acceptance criteria

Alpha 0 is the engine/privilege-boundary proof. UI polish and real application catalog breadth are explicitly out of scope.

## Acceptance criteria

| ID | Requirement |
|---|---|
| A0-01 | Build two OCI images, `deployer-web` and `deployer-agent`, with no RPM/DEB packaging. |
| A0-02 | Web is unprivileged and receives no Podman socket, systemd socket, Deployer state directory, or application-data root. |
| A0-03 | Agent exposes no TCP listener, generic shell/API proxy, broad host mount, or raw host-management endpoint; Web/Agent communication uses a private Unix socket. |
| A0-04 | Capability discovery reports Podman engine/Libpod API, rootful/rootless, Quadlet API availability, cgroup manager/version, network backend, architecture, systemd, SELinux, and approved roots. |
| A0-05 | Existing system Quadlets are discovered through native Podman Quadlet APIs and external/unmanaged definitions remain read-only. |
| A0-06 | Runtime containers correlate to generated services through `PODMAN_SYSTEMD_UNIT`, not container-name guessing. |
| A0-07 | Application directories can be created only below pre-mounted approved roots; traversal/symlink/magic-link escapes are rejected. |
| A0-08 | SELinux intent is translated without exposing arbitrary SELinux administration. |
| A0-09 | Install/update/remove create a deterministic expiring plan containing affected resources/paths, hashes, drift and a plan digest before execution. |
| A0-10 | Smoke application installs through the native Quadlet REST API with individual Quadlets and a deferred single systemd reload, then starts through systemd. |
| A0-11 | Start/stop/restart is available only for units resolved from a managed instance; no raw unit-name lifecycle endpoint exists. |
| A0-12 | Status combines Quadlet/systemd/runtime state and exposes native Podman health state where available; logs come from mapped runtime containers. |
| A0-13 | Update replaces a clean managed Quadlet; administrator drift causes refusal instead of overwrite. |
| A0-14 | A deliberately failing update restores the previous live Quadlet source and restarts the previous service; rollback failure is surfaced as a hard error. |
| A0-15 | Remove stops/removes only managed Quadlets and preserves application data by default. |
| A0-16 | The deployed smoke application continues through native systemd/Podman when Deployer is stopped and survives VM reboot. |
| A0-17 | Successful lifecycle mutations preserve persistent Markdown/JSON recovery documentation; removal retains historical documentation. Documentation root is configurable, defaulting to `/var/lib/home-server-deployer/docs`. |
| A0-18 | A versioned application-level backup contract exists without executing arbitrary catalog shell hooks; execution remains intentionally unimplemented. |
| A0-19 | Browser reaches Web only; Web reaches Agent by UDS; browser mutations require CSRF tokens; malformed/raw host-management inputs are rejected. |
| **A0-20** | **Podman 5.8.4-6.1.x compatibility:** 5.8.4 is the minimum baseline, contract tests cover 5.8.x and 6.1.x capability/API behavior, and the engine uses a stable backend abstraction. Alpha 0 uses individual Quadlets + deferred reload as the common denominator while allowing future newer-Podman native application capabilities without changing the catalog model. No dependency on Podman's private application storage representation is allowed. |
| A0-21 | Destructive integration tooling refuses to run without the dedicated test-VM marker and uses only VM-local data. |
| A0-22 | CI runs formatting/vet/unit/contract/security tests and builds both OCI images; the VM suite is separately run against the real uCore target. |

## Version test policy

The initial destructive target is the existing uCore VM with Podman 5.8.4.

CI contract tests contain separate Podman 5.8.4 and 6.1.0 version/capability fixtures. When the Fedora 45/uCore target is available, the same destructive VM suite is to be run against Podman 6.1.x without changing catalog semantics.
