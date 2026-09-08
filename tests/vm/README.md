# Alpha 0 destructive VM suite

This directory is intentionally destructive and is **not** for the production Home Server.

The harness exits unless this marker exists:

```text
/etc/home-server-deployer/test-vm
```

Use disposable/dedicated VMs with local filesystems. Do not mount production media or production application data read/write into a test VM.

## Required compatibility matrix

Alpha 0 is considered Podman-compatible only after the same core suite passes on all three current targets:

| Platform | Podman |
|---|---:|
| AlmaLinux 10.2 | 5.8.2 |
| Fedora 44 / current uCore | 5.8.4 |
| Fedora 45 | 6.1.x |

The runs may be performed sequentially and do not need to be GitHub-hosted.

The harness itself never uses `podman quadlet install --replace`. Its Deployer Quadlets are stopped/removed first with systemd reload deferred, then installed normally with `--reload-systemd=false`, followed by one daemon reload. The install sequence is repeatable after a previous or partially completed harness install.

## Core sequence

```sh
sudo touch /etc/home-server-deployer/test-vm
./tests/vm/run-alpha0.sh preflight
./tests/vm/run-alpha0.sh install-deployer
./tests/vm/run-alpha0.sh smoke-install
./tests/vm/run-alpha0.sh replacement-regression
./tests/vm/run-alpha0.sh rollback
./tests/vm/run-alpha0.sh resilience
# reboot the VM here, then:
./tests/vm/run-alpha0.sh verify-after-reboot
./tests/vm/run-alpha0.sh smoke-remove
```

`replacement-regression` first updates the installed smoke Quadlet to a longer definition containing an extra directive, then updates back to the shorter definition and verifies the removed directive is truly absent. This specifically guards against GHSA-fx76-2j3w-2mx6 / CVE-2026-19730 while exercising Deployer's safe remove + normal-install transaction on each Podman target.

`install-deployer` uses a VM-only copy of the catalog mounted read-only into Agent. This lets the `rollback` phase deliberately modify the test catalog to point at a missing image, prove rollback, then restore the test template. The normal deployment Quadlet and production catalog image remain unchanged.
