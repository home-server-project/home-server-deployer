# Home Server Deployer

> **Early Alpha development. Do not use this on a production/home server yet.**
>
> Alpha testing is intentionally limited to disposable/dedicated VMs.

Home Server Deployer is an optional, containerized manager for native Podman Quadlets. The long-term goal is an App-Store-like experience for home/lab servers without replacing Podman, systemd, or the administrator's Quadlet files.

Native Quadlet definitions remain the source of truth. If Deployer is stopped or removed, deployed applications are expected to continue running normally through Podman/systemd.

## Alpha 0

Alpha 0 is the engine and privilege-boundary proof, not the finished UI. It currently provides the implementation foundation for:

- runtime Podman capability discovery;
- Podman 5.8.2 supported floor with 5.8.2, 5.8.4, and 6.1.x capability/API contracts;
- native Quadlet discovery and generated systemd-unit mapping;
- runtime-container correlation through `PODMAN_SYSTEMD_UNIT`;
- a narrow Web -> Agent Unix-socket API;
- approved-root path containment with Linux `openat2`;
- SELinux intent translation rather than arbitrary SELinux commands;
- install/update/remove plans with immutable digests;
- deferred systemd reload transactions using normal individual Quadlets;
- administrator-drift detection before update/remove;
- update/install rollback paths;
- managed lifecycle operations and container logs;
- persistent JSON state and generated Markdown/JSON documentation;
- a typed future backup contract with no arbitrary shell hooks;
- a minimal `alpha-smoke` test catalog application.

Jellyfin is intentionally **not** part of Alpha 0. It is the first real application planned for Alpha 1.

## Architecture

```text
Browser
   |
   v
+-------------------------+
| deployer-web            |  unprivileged, no host admin sockets
+-------------------------+
   |
   | private Unix socket
   v
+-------------------------+
| deployer-agent          |  narrow privileged executor
+-------------------------+
   |                 |
   | Podman API      | systemd private D-Bus
   v                 v
/run/podman/       /run/systemd/
podman.sock        private
   |                 |
   +--------+--------+
            v
    native Quadlets/systemd
```

The Agent deliberately has **no TCP listener**, no generic shell API, no generic Podman proxy, no arbitrary unit-name endpoint, and no generic host file-management endpoint. Rootful Podman socket access is still root-equivalent; the security objective is to keep that authority inside a small auditable executor rather than pretending the socket can be made unprivileged.

See [docs/architecture.md](docs/architecture.md) and [docs/security-model.md](docs/security-model.md).

## Podman compatibility direction

The officially supported minimum is **Podman 5.8.2**. Current Alpha 0 compatibility targets are:

- AlmaLinux 10.2 / Podman 5.8.2;
- Fedora 44 / current uCore / Podman 5.8.4;
- Fedora 45 / Podman 6.1.x.

Capability detection and product support policy are intentionally separate. The Quadlet REST API family is recognized from its Podman **5.8.0** introduction point, while Home Server Deployer refuses mutations below the product-supported 5.8.2 floor.

The engine uses a stable Podman backend/capability abstraction. Alpha 0 intentionally uses one safe common-denominator update transaction on every supported version:

1. snapshot the current managed Quadlet source;
2. stop the current managed service units;
3. remove old managed Quadlets with systemd reload deferred;
4. install new Quadlets normally with systemd reload deferred;
5. perform one systemd daemon reload;
6. start/restart and verify the resolved generated units;
7. if any stage fails, restore the snapshotted source, reload once, and return the previous service to its working state.

Alpha 0 does **not** call native Quadlet `replace=true`. Podman 5.7.0 through 5.8.5 are affected by CVE-2026-19730 / GHSA-fx76-2j3w-2mx6, where replacing a longer Quadlet with a shorter one can leave stale trailing directives. Using remove + normal install avoids that path on 5.8.x and keeps identical transaction semantics on Podman 6.1.x.

Podman 6.1.x native application-install capability is detected and exposed, but Alpha 0 does not depend on it. Deployer also does not depend on Podman's private application storage representation such as older `.app` metadata or newer application subdirectories.

## Repository layout

```text
cmd/                     Web and Agent entry points
internal/api/            private Agent HTTP/JSON API
internal/engine/         plan/transaction/drift/rollback engine
internal/podman/         Libpod REST backend and compatibility profiles
internal/systemd/        direct private systemd D-Bus adapter
internal/paths/          approved-root/openat2 containment
internal/security/       platform security backend interface
internal/security/selinux/ SELinux intent backend
internal/catalog/        strict catalog loader and validation
internal/backup/         future backup-provider contract
internal/state/          atomic persistent JSON state
internal/docsgen/        persistent recovery documentation
internal/web/            primitive Alpha 0 Web UI
catalog/apps/            curated application definitions/templates
deploy/quadlet/          Deployer's own native Quadlets
tests/vm/                destructive VM-only integration harness
```

## Development

A normal developer test run is:

```sh
go test ./...
go vet ./...
```

The build has three small external Go dependencies: `go-systemd` for direct systemd D-Bus, `x/sys` for `openat2`, and `yaml.v3` for strict catalog decoding.

OCI images are built separately:

```sh
podman build -f Containerfile.agent -t home-server-deployer-agent .
podman build -f Containerfile.web -t home-server-deployer-web .
```

The final images contain only the static Go binary (plus the trusted catalog/default configuration in the Agent image). The Agent image does not include a shell, Podman CLI, or package manager.

## VM integration testing

Never run the destructive integration harness on the production Home Server.

The VM harness refuses to run unless `/etc/home-server-deployer/test-vm` exists:

```sh
sudo touch /etc/home-server-deployer/test-vm
./tests/vm/run-alpha0.sh preflight
```

Alpha 0 is considered Podman-compatible only after the same core VM suite passes on the three current targets: Podman 5.8.2, 5.8.4, and 6.1.x.

Read [tests/vm/README.md](tests/vm/README.md) before using it.

## Project

Home Server Project: <https://github.com/home-server-project>

This repository is licensed under the Apache License 2.0.
