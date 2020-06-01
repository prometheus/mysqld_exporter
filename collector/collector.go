// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"bytes"
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Exporter namespace.
	namespace = "mysql"
	// Math constant for picoseconds to seconds.
	picoSeconds = 1e12
	// Query to check whether user/table/client stats are enabled.
	userstatCheckQuery = `SHOW GLOBAL VARIABLES WHERE Variable_Name='userstat'
		OR Variable_Name='userstat_running'`
)

var logRE = regexp.MustCompile(`.+\.(\d+)$`)

func newDesc(subsystem, name, help string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, name),
		help, nil, nil,
	)
}

func parseStatus(data sql.RawBytes) (float64, bool) {
	if bytes.Equal(data, []byte("Yes")) || bytes.Equal(data, []byte("ON")) {
		return 1, true
	}
	if bytes.Equal(data, []byte("No")) || bytes.Equal(data, []byte("OFF")) {
		return 0, true
	}
	// SHOW SLAVE STATUS Slave_IO_Running can return "Connecting" which is a non-running state.
	if bytes.Equal(data, []byte("Connecting")) {
		return 0, true
	}
	// SHOW GLOBAL STATUS like 'wsrep_cluster_status' can return "Primary" or "non-Primary"/"Disconnected"
	if bytes.Equal(data, []byte("Primary")) {
		return 1, true
	}
	if strings.EqualFold(string(data), "non-Primary") || bytes.Equal(data, []byte("Disconnected")) {
		return 0, true
	}
	if logNum := logRE.Find(data); logNum != nil {
		value, err := strconv.ParseFloat(string(logNum), 64)
		return value, err == nil
	}
	value, err := strconv.ParseFloat(string(data), 64)
	return value, err == nil
}

func parsePrivilege(data sql.RawBytes) (float64, bool) {
	if bytes.Equal(data, []byte("Y")) {
		return 1, true
	}
	if bytes.Equal(data, []byte("N")) {
		return 0, true
	}
	return -1, false
}
