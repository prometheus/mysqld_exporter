#!/bin/bash
set -exo pipefail

docker_image=$1
port=$2

container_id=''

wait_start() {
    for in in {1..10}; do
        if  /usr/bin/curl -s -m 5 -f "http://localhost:${port}/metrics" > /dev/null; then
            docker_cleanup
            exit 0
        else
            sleep 1
        fi
    done
    
    exit 1
}

docker_start() {
    container_id=$(docker run -d --network mysql-test -p "${port}":"${port}" "${docker_image}" --export-multi-hosts --config-multi-hosts=test_multi_exporter.cnf)
}

docker_cleanup() {
    docker kill "${container_id}"
}

if [[ "$#" -ne 2 ]] ; then
    echo "Usage: $0 quay.io/prometheus/mysqld-exporter:v0.10.0 9104" >&2
    exit 1
fi

docker_start
wait_start
