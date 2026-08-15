#!/usr/bin/env bash
set -euo pipefail

image="${1:?usage: container-smoke.sh <image>}"
container_name="go-scaffold-template-smoke-${GITHUB_RUN_ID:-local}-$$"
volume_name="${container_name}-data"

cleanup() {
  docker rm -f "${container_name}" >/dev/null 2>&1 || true
  docker volume rm "${volume_name}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker volume create "${volume_name}" >/dev/null
docker run --rm --network host \
  --mount "type=volume,src=${volume_name},dst=/app/.data" \
  "${image}" db migrate up

docker run --detach --name "${container_name}" --network host \
  --read-only --tmpfs /tmp:rw,noexec,nosuid,size=16m \
  --cap-drop ALL --security-opt no-new-privileges \
  --mount "type=volume,src=${volume_name},dst=/app/.data" \
  "${image}" >/dev/null

for _ in $(seq 1 40); do
  if curl --fail --silent http://127.0.0.1:9090/readyz >/dev/null; then
    break
  fi
  sleep 0.5
done
curl --fail --silent http://127.0.0.1:9090/readyz >/dev/null
curl --fail --silent http://127.0.0.1:9090/build | grep -F '"version"' >/dev/null

if [[ "$(docker inspect --format '{{.Config.User}}' "${container_name}")" != "nonroot:nonroot" ]]; then
  echo "container user is not nonroot:nonroot" >&2
  exit 1
fi
if [[ "$(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' "${container_name}")" != "true" ]]; then
  echo "container root filesystem is writable" >&2
  exit 1
fi
if docker exec "${container_name}" /bin/sh -c true >/dev/null 2>&1; then
  echo "runtime image unexpectedly contains /bin/sh" >&2
  exit 1
fi

docker stop --time 15 "${container_name}" >/dev/null
exit_code="$(docker inspect --format '{{.State.ExitCode}}' "${container_name}")"
if [[ "${exit_code}" != "0" ]]; then
  docker logs "${container_name}" >&2
  exit 1
fi
