#!/usr/bin/env bash
set -euo pipefail

# No host port mapping anymore — we query the exporter from inside the docker network.
IMAGE_TAG="mysqld-exporter:local"
NETWORK_NAME="mysql-test"
MYSQL_ADDR="mysql:3306"
EXPORTER_NAME="mysqld_exporter_test_exporter"
METRICS_URL_NET="http://${EXPORTER_NAME}:9104/metrics"

declare -A TESTS

# Core
TESTS["collect.global_status"]="^mysql_global_status_"
TESTS["collect.global_variables"]="^mysql_global_variables_"

# SYS schema
TESTS["collect.sys.user_summary"]="^mysql_sys_statements_total"
TESTS["collect.sys.user_summary_by_statement_latency"]="^mysql_sys_user_summary_by_statement_latency"
TESTS["collect.sys.user_summary_by_statement_type"]="^mysql_sys_user_summary_by_statement_type"

# INFORMATION_SCHEMA
TESTS["collect.info_schema.innodb_cmp"]="^mysql_info_schema_innodb_cmp"
TESTS["collect.info_schema.innodb_cmpmem"]="^mysql_info_schema_innodb_cmpmem"
TESTS["collect.info_schema.innodb_metrics"]="^mysql_info_schema_innodb_metrics"
TESTS["collect.info_schema.processlist"]="^mysql_info_schema_processlist_threads"

# ENGINE / replication (will be present but may be empty on single-instance; still safe to grep family)
TESTS["collect.engine_innodb_status"]="^mysql_engine_innodb_queries_in_queue"

# Performance Schema (commonly available on 5.7/8.x with P_S on)
TESTS["collect.perf_schema.eventsstatements"]="^mysql_perf_schema_events_statements_total"
TESTS["collect.perf_schema.eventsstatementssum"]="^mysql_perf_schema_events_statements_sum_total"

LOG_DIR="${LOG_DIR:-./_testlogs}"
mkdir -p "${LOG_DIR}"

log() { printf '%s\n' "$*" >&2; }
need() { command -v "$1" >/dev/null 2>&1 || { log "Missing: $1"; exit 1; }; }
compose() { if docker compose version >/dev/null 2>&1; then docker compose "$@"; else docker-compose "$@"; fi; }

# Curl inside the docker network (avoids host port binds).
curl_in_net() {
  docker run --rm --network "${NETWORK_NAME}" curlimages/curl:8.8.0 \
    curl -sS -m 3 -f "$1"
}

wait_for_metrics_net() {
  local url="$1" retries="${2:-30}" sleep_s="${3:-1}"
  for _ in $(seq 1 "$retries"); do
    if curl_in_net "$url" >/dev/null 2>&1; then return 0; fi
    sleep "$sleep_s"
  done
  return 1
}

dns_probe_mysql() {
  for _ in $(seq 1 30); do
    if docker run --rm --network "${NETWORK_NAME}" busybox:1.36 nslookup mysql >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

exporter_logs() {
  local name="$1"
  log "---- exporter container state ----"
  docker inspect -f 'Name={{.Name}} Status={{.State.Status}} ExitCode={{.State.ExitCode}} OOMKilled={{.State.OOMKilled}} Error={{.State.Error}} StartedAt={{.State.StartedAt}} FinishedAt={{.State.FinishedAt}}' "$name" 2>/dev/null || true
  log "---- exporter logs (last 200 lines) ----"
  docker logs --tail=200 "$name" 2>&1 || true
  log "----------------------------------------"
}

mysql_logs() { compose logs --no-color --tail=200 mysql || true; }

build_binary() {
  export CGO_ENABLED=0
  mkdir -p .build/linux-amd64/
  go build -o .build/linux-amd64/mysqld_exporter
}

build_local_image() {
  log "▶ Building local exporter image: ${IMAGE_TAG}"
  docker build -t "${IMAGE_TAG}" .
}

up_stack() {
  log "▶ Bringing up MySQL…"
  compose up -d mysql

  log "⏳ Waiting for MySQL to be healthy…"
  for _ in $(seq 1 60); do
    st="$(docker inspect -f '{{.State.Health.Status}}' "$(compose ps -q mysql)" 2>/dev/null || echo "unknown")"
    [[ "$st" == "healthy" ]] && break
    sleep 1
  done

  log "⏳ Probing DNS for 'mysql' on '${NETWORK_NAME}'…"
  dns_probe_mysql || { log "DNS for 'mysql' not resolving"; docker network inspect "${NETWORK_NAME}" || true; exit 1; }

  log "▶ Ensuring monitoring/app users and grants…"
  compose exec -T mysql sh -lc '
    mysql -uroot -prootpass <<SQL
CREATE USER IF NOT EXISTS '\''exporter'\''@'\''%'\'' IDENTIFIED BY '\''exporter'\'';
ALTER USER '\''exporter'\''@'\''%'\'' IDENTIFIED WITH caching_sha2_password BY '\''exporter'\'';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO '\''exporter'\''@'\''%'\''; 

CREATE USER IF NOT EXISTS '\''app'\''@'\''%'\'' IDENTIFIED BY '\''app'\'';
CREATE DATABASE IF NOT EXISTS testdb;
GRANT ALL PRIVILEGES ON testdb.* TO '\''app'\''@'\''%'\''; 

FLUSH PRIVILEGES;
SQL'
  

  log "▶ Running seed workload…"
  compose run --rm seed || true
}

down_stack() {
  log "▶ Tearing down stack…"
  compose down -v
}

make_temp_cnf() {
  local dir; dir="$(mktemp -d)"
  local path="${dir}/my.cnf"
  cat > "${path}" <<EOF
[client]
user=exporter
password=exporter
host=mysql
port=3306
EOF
  printf '%s' "${path}"
}

run_exporter_with_flag() {
  local flag="$1"
  docker rm -f "${EXPORTER_NAME}" >/dev/null 2>&1 || true

  local cnf_path; cnf_path="$(make_temp_cnf)"

  log "▶ Starting exporter with --${flag}"
  local cid
  cid="$(
    docker run -d \
      --name "${EXPORTER_NAME}" \
      --network "${NETWORK_NAME}" \
      -v "${cnf_path}:/cfg/my.cnf:ro" \
      "${IMAGE_TAG}" \
      --web.listen-address=":9104" \
      --log.level=debug \
      --config.my-cnf=/cfg/my.cnf \
      --mysqld.address="${MYSQL_ADDR}" \
      --"${flag}"
  )"

  # Tail logs to a local file (background)
  LOG_FILE="${LOG_DIR}/exporter_${flag//./_}.log"
  docker logs -f "${EXPORTER_NAME}" >"${LOG_FILE}" 2>&1 &
  TAIL_PID="$!"

  # Wait for /metrics inside the docker network
  if ! wait_for_metrics_net "${METRICS_URL_NET}" 30 1; then
    echo "${flag}: FAIL (exporter did not become ready)"
    log "---- exporter log file tail (${LOG_FILE}) ----"
    tail -n 200 "${LOG_FILE}" 2>/dev/null || true
    exporter_logs "${EXPORTER_NAME}"
    log "---- mysql logs (tail) ----"
    mysql_logs
    return 1
  fi

  printf '%s' "$cid"
}

stop_exporter_tail() { [[ -n "${TAIL_PID:-}" ]] && kill "${TAIL_PID}" >/dev/null 2>&1 || true; unset TAIL_PID; }

smoke_test_mysql_auth() {
  log "▶ Smoke-testing exporter credentials from client…"
  docker run --rm --network "${NETWORK_NAME}" mysql:8.4 \
    mysql -h mysql -uexporter -pexporter -e 'SELECT @@version;' >/dev/null
}

# Get metrics content from inside the network (avoids host port)
get_metrics_net() {
  curl_in_net "${METRICS_URL_NET}"
}

test_flag() {
  local flag="$1" pattern="$2"

  local cid
  if ! cid="$(run_exporter_with_flag "$flag")"; then
    stop_exporter_tail
    return 1
  fi

  LOG_FILE="${LOG_DIR}/exporter_${flag//./_}.log"

  # Pull metrics and test
  if get_metrics_net | grep -E "${pattern}" >/dev/null; then
    echo "${flag}: PASS"
    stop_exporter_tail
    docker rm -f "${EXPORTER_NAME}" >/dev/null 2>&1 || true
    return 0
  else
    echo "${flag}: FAIL (pattern not found: ${pattern})"
    log "---- first 30 mysql_* metrics ----"
    get_metrics_net | grep '^mysql_' | head -n 30 || true
    log "---- exporter log file tail ----"
    tail -n 200 "${LOG_FILE}" 2>/dev/null || true
    exporter_logs "${EXPORTER_NAME}"
    stop_exporter_tail
    docker rm -f "${EXPORTER_NAME}" >/dev/null 2>&1 || true
    return 1
  fi
}

need docker; need curl
build_binary
build_local_image
up_stack

if ! smoke_test_mysql_auth; then
  log "Auth FAILED"
  mysql_logs
  down_stack
  exit 1
else
  log "Auth OK"
fi

pass=0
fail=0
for flag in ${!TESTS[@]}; do
  if test_flag "$flag" "${TESTS[${flag}]}"; then
    pass=$((pass+1))
  else
    fail=$((fail+1))
 fi
done

echo; echo "==== Summary ===="; echo "PASS: ${pass}"; echo "FAIL: ${fail}"
down_stack
[[ "$fail" -eq 0 ]]

