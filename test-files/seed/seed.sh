#!/usr/bin/env bash
set -euo pipefail

host="${MYSQL_HOST:-mysql}"
user="${MYSQL_USER:-app}"
pass="${MYSQL_PASSWORD:-app}"
db="${MYSQL_DATABASE:-testdb}"

# Helper to run mysql client quietly
mysqlq() {
  mysql -h "$host" -u "$user" "-p$pass" -D "$db" -ss -N -e "$1" >/dev/null 2>&1
}

echo "Seed: waiting for DNS for host '${host}'…"
for i in {1..60}; do
  if getent hosts "$host" > /dev/null 2>&1; then
    echo "Seed: DNS OK ($(getent hosts "$host" | awk '{print $1}' | head -n1))"
    break
  fi
  sleep 1
  [[ $i -eq 60 ]] && { echo "Seed: DNS for '$host' not found"; exit 1; }
done

echo "Seed: waiting for MySQL TCP on ${host}:3306…"
for i in {1..60}; do
  if mysqladmin ping -h "$host" -u"$user" -p"$pass" --silent 2>/dev/null; then
    echo "Seed: MySQL is up."
    break
  fi
  sleep 1
  [[ $i -eq 60 ]] && { echo "Seed: MySQL not reachable"; exit 1; }
done

# Ensure table exists (idempotent)
mysqlq "CREATE TABLE IF NOT EXISTS t1 (id INT PRIMARY KEY AUTO_INCREMENT, v INT, ts TIMESTAMP DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB;"

# Generate some mixed statements under the 'app' user
for i in $(seq 1 200); do
  mysqlq "INSERT INTO t1 (v) VALUES (FLOOR(RAND()*1000));"
  mysqlq "SELECT COUNT(*) FROM t1;"
  mysqlq "UPDATE t1 SET v = v + 1 WHERE id % 10 = 0;"
  mysqlq "SELECT SLEEP(0.01);"
done

echo "Seed workload finished."
