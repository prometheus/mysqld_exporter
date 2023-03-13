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

// Scrape `information_schema.processlist`.

package collector

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const infoSchemaProcesslistQuery = `
		  SELECT
		    user,
		    SUBSTRING_INDEX(host, ':', 1) AS host,
		    COALESCE(command, '') AS command,
		    COALESCE(state, '') AS state,
		    COUNT(*) AS processes,
		    SUM(time) AS seconds
		  FROM information_schema.processlist
		  WHERE ID != connection_id()
		    AND TIME >= %d
		  GROUP BY user, SUBSTRING_INDEX(host, ':', 1), command, state
	`

// Tunable flags.
var (
	processlistMinTime = kingpin.Flag(
		"collect.info_schema.processlist.min_time",
		"Minimum time a thread must be in each state to be counted",
	).Default("0").Int()
	processesByUserFlag = kingpin.Flag(
		"collect.info_schema.processlist.processes_by_user",
		"Enable collecting the number of processes by user",
	).Default("true").Bool()
	processesByHostFlag = kingpin.Flag(
		"collect.info_schema.processlist.processes_by_host",
		"Enable collecting the number of processes by host",
	).Default("true").Bool()
)

// Metric descriptors.
var (
	processlistCountDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "processlist_threads"),
		"The number of threads split by current state.",
		[]string{"command", "state"}, nil)
	processlistTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "processlist_seconds"),
		"The number of seconds threads have used split by current state.",
		[]string{"command", "state"}, nil)
	processesByUserDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "processlist_processes_by_user"),
		"The number of processes by user.",
		[]string{"mysql_user"}, nil)
	processesByHostDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "processlist_processes_by_host"),
		"The number of processes by host.",
		[]string{"client_host"}, nil)
)

// ScrapeProcesslist collects from `information_schema.processlist`.
type ScrapeProcesslist struct{}

// Name of the Scraper. Should be unique.
func (ScrapeProcesslist) Name() string {
	return informationSchema + ".processlist"
}

// Help describes the role of the Scraper.
func (ScrapeProcesslist) Help() string {
	return "Collect current thread state counts from the information_schema.processlist"
}

// Version of MySQL from which scraper is available.
func (ScrapeProcesslist) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeProcesslist) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	processQuery := fmt.Sprintf(
		infoSchemaProcesslistQuery,
		*processlistMinTime,
	)
	processlistRows, err := db.QueryContext(ctx, processQuery)
	if err != nil {
		return err
	}
	defer processlistRows.Close()

	var (
		user    string
		host    string
		command string
		state   string
		count   uint32
		time    uint32
	)
	// Define maps
	stateCounts := make(map[string]map[string]uint32)
	stateTime := make(map[string]map[string]uint32)
	stateHostCounts := make(map[string]uint32)
	stateUserCounts := make(map[string]uint32)

	for processlistRows.Next() {
		err = processlistRows.Scan(&user, &host, &command, &state, &count, &time)
		if err != nil {
			return err
		}
		command = sanitizeState(command)
		state = sanitizeState(state)
		if host == "" {
			host = "unknown"
		}

		// Init maps
		if _, ok := stateCounts[command]; !ok {
			stateCounts[command] = make(map[string]uint32)
			stateTime[command] = make(map[string]uint32)
		}
		if _, ok := stateCounts[command][state]; !ok {
			stateCounts[command][state] = 0
			stateTime[command][state] = 0
		}
		if _, ok := stateHostCounts[host]; !ok {
			stateHostCounts[host] = 0
		}
		if _, ok := stateUserCounts[user]; !ok {
			stateUserCounts[user] = 0
		}

		stateCounts[command][state] += count
		stateTime[command][state] += time
		stateHostCounts[host] += count
		stateUserCounts[user] += count
	}

	for _, command := range sortedMapKeys(stateCounts) {
		for _, state := range sortedMapKeys(stateCounts[command]) {
			ch <- prometheus.MustNewConstMetric(processlistCountDesc, prometheus.GaugeValue, float64(stateCounts[command][state]), command, state)
			ch <- prometheus.MustNewConstMetric(processlistTimeDesc, prometheus.GaugeValue, float64(stateTime[command][state]), command, state)
		}
	}

	if *processesByHostFlag {
		for _, host := range sortedMapKeys(stateHostCounts) {
			ch <- prometheus.MustNewConstMetric(processesByHostDesc, prometheus.GaugeValue, float64(stateHostCounts[host]), host)
		}
	}
	if *processesByUserFlag {
		for _, user := range sortedMapKeys(stateUserCounts) {
			ch <- prometheus.MustNewConstMetric(processesByUserDesc, prometheus.GaugeValue, float64(stateUserCounts[user]), user)
		}
	}

	return nil
}

func sortedMapKeys(m interface{}) []string {
	v := reflect.ValueOf(m)
	keys := make([]string, 0, len(v.MapKeys()))
	for _, key := range v.MapKeys() {
		keys = append(keys, key.String())
	}
	sort.Strings(keys)
	return keys
}

func sanitizeState(state string) string {
	if state == "" {
		state = "unknown"
	}
	state = strings.ToLower(state)
	replacements := map[string]string{
		";": "",
		",": "",
		":": "",
		".": "",
		"(": "",
		")": "",
		" ": "_",
		"-": "_",
	}
	for r := range replacements {
		state = strings.Replace(state, r, replacements[r], -1)
	}
	return state
}

// check interface
var _ Scraper = ScrapeProcesslist{}
