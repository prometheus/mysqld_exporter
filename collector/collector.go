package collector

import (
	"bytes"
	"database/sql"
	"regexp"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Exporter namespace.
	namespace = "mysql"
	// Math constant for picoseconds to seconds.
	picoSeconds = 1e12
)

var logRE = regexp.MustCompile(`.+\.(\d+)$`)

func newDesc(subsystem, name, help string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, name),
		help, nil, nil,
	)
}

func parseStatus(data sql.RawBytes) (float64, bool) {
	if bytes.Compare(data, []byte("Yes")) == 0 || bytes.Compare(data, []byte("ON")) == 0 {
		return 1, true
	}
	if bytes.Compare(data, []byte("No")) == 0 || bytes.Compare(data, []byte("OFF")) == 0 {
		return 0, true
	}
	// SHOW SLAVE STATUS Slave_IO_Running can return "Connecting" which is a non-running state.
	if bytes.Compare(data, []byte("Connecting")) == 0 {
		return 0, true
	}
	if logNum := logRE.Find(data); logNum != nil {
		value, err := strconv.ParseFloat(string(logNum), 64)
		return value, err == nil
	}
	value, err := strconv.ParseFloat(string(data), 64)
	return value, err == nil
}
