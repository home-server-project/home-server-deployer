#!/usr/bin/env bash
set -euo pipefail

MARKER=/etc/home-server-deployer/test-vm
REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
TEST_ROOT=/var/lib/home-server-deployer-vm
TEST_CATALOG=${TEST_ROOT}/catalog
AGENT_TEST_QUADLET=${TEST_ROOT}/home-server-deployer-agent.container
RUNTIME_VOLUME=home-server-deployer-runtime
MIN_PODMAN_VERSION=5.8.2

fail() { echo "ERROR: $*" >&2; exit 1; }
require_vm() { [[ -f "$MARKER" ]] || fail "refusing destructive test: $MARKER is missing"; }
version_ge() { [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]; }

socket_path() {
  local mount
  mount=$(sudo podman volume inspect "$RUNTIME_VOLUME" --format '{{.Mountpoint}}')
  printf '%s/agent.sock\n' "$mount"
}

wait_agent() {
  local sock i
  sock=$(socket_path)
  for i in $(seq 1 30); do
    if sudo test -S "$sock" && sudo curl -fsS --unix-socket "$sock" http://localhost/v1/health >/dev/null; then
      return 0
    fi
    sleep 1
  done
  fail "Agent Unix socket did not become ready"
}

api_get() {
  local path=$1 sock
  sock=$(socket_path)
  sudo curl -fsS --unix-socket "$sock" "http://localhost${path}"
}

api_post() {
  local path=$1 payload=$2 sock
  sock=$(socket_path)
  sudo curl -fsS --unix-socket "$sock" -H 'Content-Type: application/json' -X POST -d "$payload" "http://localhost${path}"
}

plan_id() {
  sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -n1
}

execute_update() {
  local response id
  response=$(api_post /v1/plans '{"operation":"update","appId":"alpha-smoke","instanceId":"alpha-smoke"}')
  id=$(printf '%s' "$response" | plan_id)
  [[ -n "$id" ]] || fail "could not extract update plan ID: $response"
  api_post "/v1/plans/${id}/execute" '{}'
}

preflight() {
  require_vm
  command -v podman >/dev/null || fail "podman missing"
  command -v curl >/dev/null || fail "curl missing"
  command -v getenforce >/dev/null || fail "getenforce missing"
  [[ "$(getenforce)" == "Enforcing" ]] || fail "SELinux must be Enforcing for Alpha 0 VM validation"
  local pv rootless
  pv=$(podman --version | awk '{print $3}')
  version_ge "$pv" "$MIN_PODMAN_VERSION" || fail "Podman $pv is below the Home Server Deployer $MIN_PODMAN_VERSION supported floor"
  rootless=$(sudo podman info --format '{{.Host.Security.Rootless}}')
  [[ "$rootless" == "false" ]] || fail "rootful Podman is required for current Alpha 0"
  sudo podman quadlet list >/dev/null
  sudo test -S /run/podman/podman.sock || fail "/run/podman/podman.sock missing"
  sudo test -S /run/systemd/private || fail "/run/systemd/private missing"
  echo "preflight OK: Podman $pv, rootful, SELinux enforcing"
}

remove_existing_deployer_quadlets() {
  # Stop generated services before source removal so the harness never needs
  # native quadlet --replace or forced source replacement. --ignore makes the
  # sequence repeatable after a clean run or a partially completed prior run.
  sudo systemctl stop home-server-deployer-web.service home-server-deployer-agent.service 2>/dev/null || true
  sudo systemctl stop home-server-deployer-runtime-volume.service 2>/dev/null || true
  sudo podman quadlet rm --reload-systemd=false --ignore home-server-deployer-web.container >/dev/null
  sudo podman quadlet rm --reload-systemd=false --ignore home-server-deployer-agent.container >/dev/null
  sudo podman quadlet rm --reload-systemd=false --ignore home-server-deployer-runtime.volume >/dev/null
}

install_deployer() {
  require_vm
  preflight
  sudo mkdir -p /var/lib/home-server-deployer /var/lib/home-server-apps "$TEST_ROOT"
  sudo rm -rf "$TEST_CATALOG"
  sudo cp -a "$REPO_ROOT/catalog" "$TEST_CATALOG"

  sudo awk -v cat="$TEST_CATALOG" '
    { print }
    $0 == "Volume=home-server-deployer-runtime.volume:/run/home-server-deployer" {
      print "Volume=" cat ":/usr/share/home-server-deployer/catalog:ro"
    }
  ' "$REPO_ROOT/deploy/quadlet/home-server-deployer-agent.container" | sudo tee "$AGENT_TEST_QUADLET" >/dev/null

  remove_existing_deployer_quadlets
  sudo podman quadlet install --reload-systemd=false "$REPO_ROOT/deploy/quadlet/home-server-deployer-runtime.volume"
  sudo podman quadlet install --reload-systemd=false "$AGENT_TEST_QUADLET"
  sudo podman quadlet install --reload-systemd=false "$REPO_ROOT/deploy/quadlet/home-server-deployer-web.container"
  sudo systemctl daemon-reload
  sudo systemctl start home-server-deployer-agent.service
  sudo systemctl start home-server-deployer-web.service
  wait_agent
  api_get /v1/capabilities
  echo
  echo "Deployer installed in VM test mode"
}

smoke_install() {
  require_vm
  wait_agent
  if api_get /v1/instances 2>/dev/null | grep -q '"id":"alpha-smoke"'; then
    fail "alpha-smoke is already managed on this VM; run smoke-remove or reset the disposable VM before smoke-install"
  fi
  local response id
  response=$(api_post /v1/plans '{"operation":"install","appId":"alpha-smoke","instanceId":"alpha-smoke","parameters":{"data-subpath":"alpha-smoke"}}')
  id=$(printf '%s' "$response" | plan_id)
  [[ -n "$id" ]] || fail "could not extract install plan ID: $response"
  api_post "/v1/plans/${id}/execute" '{}'
  echo
  sudo systemctl is-active --quiet alpha-smoke.service || fail "alpha-smoke.service is not active"
  sudo podman quadlet print alpha-smoke.container | grep -q 'Deployer-Instance: alpha-smoke' || fail "managed source header missing"
  sudo test -d /var/lib/home-server-apps/alpha-smoke || fail "approved data directory missing"
  sudo test -f /var/lib/home-server-deployer/docs/alpha-smoke/README.md || fail "generated documentation missing"
  echo "smoke install OK"
}

replacement_regression() {
  require_vm
  wait_agent
  sudo systemctl is-active --quiet alpha-smoke.service || fail "smoke app must be installed before replacement regression"
  local tpl backup marker
  tpl="$TEST_CATALOG/apps/alpha-smoke/alpha-smoke.container.tmpl"
  backup="${tpl}.replace-regression-good"
  marker='HOME_SERVER_DEPLOYER_REPLACE_REGRESSION=1'
  sudo cp "$tpl" "$backup"
  restore_template() { sudo cp "$backup" "$tpl" 2>/dev/null || true; sudo rm -f "$backup" 2>/dev/null || true; }
  trap restore_template EXIT

  sudo sed -i "/^Exec=sleep infinity$/a Environment=${marker}" "$tpl"
  execute_update >/dev/null
  sudo podman quadlet print alpha-smoke.container | grep -q "$marker" || fail "long regression Quadlet did not contain marker directive"

  sudo cp "$backup" "$tpl"
  execute_update >/dev/null
  if sudo podman quadlet print alpha-smoke.container | grep -q "$marker"; then
    fail "removed trailing directive survived the safe remove+install update transaction"
  fi

  restore_template
  trap - EXIT
  sudo systemctl is-active --quiet alpha-smoke.service || fail "replacement regression left smoke service inactive"
  echo "shorter-Quadlet replacement regression OK"
}

rollback_test() {
  require_vm
  wait_agent
  sudo systemctl is-active --quiet alpha-smoke.service || fail "smoke app must be installed before rollback test"
  local tpl backup response id sock body code
  tpl="$TEST_CATALOG/apps/alpha-smoke/alpha-smoke.container.tmpl"
  backup="${tpl}.good"
  sudo cp "$tpl" "$backup"
  restore_template() { sudo cp "$backup" "$tpl" 2>/dev/null || true; sudo rm -f "$backup" 2>/dev/null || true; }
  trap restore_template EXIT
  sudo sed -i 's#Image=docker.io/library/alpine:3.22#Image=localhost/home-server-deployer-intentionally-missing:never#' "$tpl"
  response=$(api_post /v1/plans '{"operation":"update","appId":"alpha-smoke","instanceId":"alpha-smoke"}')
  id=$(printf '%s' "$response" | plan_id)
  [[ -n "$id" ]] || fail "could not extract update plan ID"
  sock=$(socket_path)
  set +e
  body=$(sudo curl -sS --unix-socket "$sock" -H 'Content-Type: application/json' -X POST -d '{}' -w $'\n%{http_code}' "http://localhost/v1/plans/${id}/execute")
  set -e
  code=$(printf '%s\n' "$body" | tail -n1)
  [[ "$code" -ge 400 ]] || fail "intentionally broken update unexpectedly succeeded"
  restore_template
  trap - EXIT
  sudo systemctl is-active --quiet alpha-smoke.service || fail "rollback did not restore active service"
  sudo podman quadlet print alpha-smoke.container | grep -q 'Image=docker.io/library/alpine:3.22' || fail "rollback did not restore prior Quadlet source"
  echo "broken-update rollback OK (HTTP $code from injected failure)"
}

resilience() {
  require_vm
  sudo systemctl is-active --quiet alpha-smoke.service || fail "smoke app must be active"
  sudo systemctl stop home-server-deployer-web.service home-server-deployer-agent.service
  sudo systemctl is-active --quiet alpha-smoke.service || fail "application stopped when Deployer stopped"
  sudo systemctl start home-server-deployer-agent.service home-server-deployer-web.service
  wait_agent
  api_get /v1/instances | grep -q 'alpha-smoke' || fail "Deployer did not reconstruct persistent instance state after restart"
  echo "Deployer restart independence OK"
}

verify_after_reboot() {
  require_vm
  sudo systemctl is-active --quiet alpha-smoke.service || fail "smoke application did not survive VM reboot"
  sudo systemctl is-active --quiet home-server-deployer-agent.service || fail "Agent not active after reboot"
  wait_agent
  api_get /v1/instances/alpha-smoke | grep -q 'alpha-smoke.service' || fail "status mapping missing after reboot"
  echo "post-reboot persistence OK"
}

smoke_remove() {
  require_vm
  wait_agent
  local response id
  response=$(api_post /v1/plans '{"operation":"remove","appId":"alpha-smoke","instanceId":"alpha-smoke"}')
  id=$(printf '%s' "$response" | plan_id)
  [[ -n "$id" ]] || fail "could not extract remove plan ID"
  api_post "/v1/plans/${id}/execute" '{}'
  echo
  if sudo podman quadlet list | grep -q 'alpha-smoke.container'; then fail "smoke Quadlet still installed"; fi
  sudo test -d /var/lib/home-server-apps/alpha-smoke || fail "application data was deleted during remove"
  sudo test -f /var/lib/home-server-deployer/docs/alpha-smoke/README.md || fail "historical docs were deleted"
  sudo grep -q 'Removed:' /var/lib/home-server-deployer/docs/alpha-smoke/README.md || fail "removal not recorded in docs"
  echo "smoke remove OK; data and documentation preserved"
}

case "${1:-}" in
  preflight) preflight ;;
  install-deployer) install_deployer ;;
  smoke-install) smoke_install ;;
  replacement-regression) replacement_regression ;;
  rollback) rollback_test ;;
  resilience) resilience ;;
  verify-after-reboot) verify_after_reboot ;;
  smoke-remove) smoke_remove ;;
  *) echo "usage: $0 {preflight|install-deployer|smoke-install|replacement-regression|rollback|resilience|verify-after-reboot|smoke-remove}" >&2; exit 2 ;;
esac
