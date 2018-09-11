// Scrape `information_schema.processlist`.

package collector

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const infoSchemaProcesslistQuery = `
		SELECT COALESCE(command,''),COALESCE(state,''),count(*),sum(time)
		  FROM information_schema.processlist
		  WHERE ID != connection_id()
		    AND TIME >= %d
		  GROUP BY command,state
		  ORDER BY null
		`
const infoSchemaProcessesByUserQuery = `
		SELECT user, count(user) processes
		  FROM information_schema.processlist
		  GROUP BY user
		`
const infoSchemaProcessesByHostQuery = `
		SELECT LEFT(host, LOCATE(':', host) - 1) host, count(*) processes
		FROM information_schema.processlist
		GROUP BY LEFT(host, LOCATE(':', host) - 1)
	`

// Tunable flags.
var (
	processlistMinTime = kingpin.Flag(
		"collect.info_schema.processlist.min_time",
		"Minimum time a thread must be in each state to be counted",
	).Default("0").Int()
	processesByUserFlag = kingpin.Flag(
		"collect.info_schema.processlist.processes_by_user",
		"Collect the number of processes by user",
	).Default("false").Bool()
	processesByHostFlag = kingpin.Flag(
		"collect.info_schema.processlist.processes_by_host",
		"Collect the number of processes by host",
	).Default("false").Bool()
)

// Metric descriptors.
var (
	processlistCountDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "threads"),
		"The number of threads (connections) split by current state.",
		[]string{"state"}, nil)
	processlistTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "threads_seconds"),
		"The number of seconds threads (connections) have used split by current state.",
		[]string{"state"}, nil)
	processesByUserDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "processes_by_user"),
		"The number of processes by user.",
		[]string{"src_user"}, nil)
	processesByHostDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "processes_by_host"),
		"The number of processes by host.",
		[]string{"src_host"}, nil)
)

// whitelist for connection/process states in SHOW PROCESSLIST
// tokudb uses the state column for "Queried about _______ rows"
var (
	// TODO: might need some more keys for other MySQL versions or other storage engines
	// see https://dev.mysql.com/doc/refman/5.7/en/general-thread-states.html
	threadStateCounterMap = map[string]uint32{
		"after create":              uint32(0),
		"altering table":            uint32(0),
		"analyzing":                 uint32(0),
		"checking permissions":      uint32(0),
		"checking table":            uint32(0),
		"cleaning up":               uint32(0),
		"closing tables":            uint32(0),
		"converting heap to myisam": uint32(0),
		"copying to tmp table":      uint32(0),
		"creating sort index":       uint32(0),
		"creating table":            uint32(0),
		"creating tmp table":        uint32(0),
		"deleting":                  uint32(0),
		"executing":                 uint32(0),
		"execution of init_command": uint32(0),
		"end":                     uint32(0),
		"freeing items":           uint32(0),
		"flushing tables":         uint32(0),
		"fulltext initialization": uint32(0),
		"idle":                      uint32(0),
		"init":                      uint32(0),
		"killed":                    uint32(0),
		"waiting for lock":          uint32(0),
		"logging slow query":        uint32(0),
		"login":                     uint32(0),
		"manage keys":               uint32(0),
		"opening tables":            uint32(0),
		"optimizing":                uint32(0),
		"preparing":                 uint32(0),
		"reading from net":          uint32(0),
		"removing duplicates":       uint32(0),
		"removing tmp table":        uint32(0),
		"reopen tables":             uint32(0),
		"repair by sorting":         uint32(0),
		"repair done":               uint32(0),
		"repair with keycache":      uint32(0),
		"replication master":        uint32(0),
		"rolling back":              uint32(0),
		"searching rows for update": uint32(0),
		"sending data":              uint32(0),
		"sorting for group":         uint32(0),
		"sorting for order":         uint32(0),
		"sorting index":             uint32(0),
		"sorting result":            uint32(0),
		"statistics":                uint32(0),
		"updating":                  uint32(0),
		"waiting for tables":        uint32(0),
		"waiting for table flush":   uint32(0),
		"waiting on cond":           uint32(0),
		"writing to net":            uint32(0),
		"other":                     uint32(0),
	}
	threadStateMapping = map[string]string{
		"user sleep":                               "idle",
		"creating index":                           "altering table",
		"committing alter table to storage engine": "altering table",
		"discard or import tablespace":             "altering table",
		"rename":                                   "altering table",
		"setup":                                    "altering table",
		"renaming result table":                    "altering table",
		"preparing for alter table":                "altering table",
		"copying to group table":                   "copying to tmp table",
		"copy to tmp table":                        "copying to tmp table",
		"query end":                                "end",
		"update":                                   "updating",
		"updating main table":                      "updating",
		"updating reference tables":                "updating",
		"system lock":                              "waiting for lock",
		"user lock":                                "waiting for lock",
		"table lock":                               "waiting for lock",
		"deleting from main table":                 "deleting",
		"deleting from reference tables":           "deleting",
	}
)

func processesByUser(db *sql.DB, ch chan<- prometheus.Metric, query string, i int) error {
	connectionsByUserRows, err := db.Query(infoSchemaProcessesByUserQuery)
	if err != nil {
		return err
	}
	defer connectionsByUserRows.Close()

	var (
		user      string
		processes uint32
	)

	for connectionsByUserRows.Next() {
		err = connectionsByUserRows.Scan(&user, &processes)
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(processesByUserDesc, prometheus.GaugeValue, float64(processes), user)
	}

	return nil
}

func processesByHost(db *sql.DB, ch chan<- prometheus.Metric, query string, i int) error {
	connectionsByHostRows, err := db.Query(infoSchemaProcessesByHostQuery)
	if err != nil {
		return err
	}
	defer connectionsByHostRows.Close()

	var (
		host      string
		processes uint32
	)

	for connectionsByHostRows.Next() {
		err = connectionsByHostRows.Scan(&host, &processes)
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(processesByHostDesc, prometheus.GaugeValue, float64(processes), host)
	}

	return nil
}

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

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeProcesslist) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
	if *processesByUserFlag == true {
		processesByUser(db, ch, infoSchemaProcessesByUserQuery, 1)
	}
	if *processesByHostFlag == true {
		processesByHost(db, ch, infoSchemaProcessesByHostQuery, 1)
	}

	processQuery := fmt.Sprintf(
		infoSchemaProcesslistQuery,
		*processlistMinTime,
	)
	processlistRows, err := db.Query(processQuery)
	if err != nil {
		return err
	}
	defer processlistRows.Close()

	var (
		command string
		state   string
		count   uint32
		time    uint32
	)
	stateCounts := make(map[string]uint32, len(threadStateCounterMap))
	stateTime := make(map[string]uint32, len(threadStateCounterMap))
	for k, v := range threadStateCounterMap {
		stateCounts[k] = v
		stateTime[k] = v
	}

	for processlistRows.Next() {
		err = processlistRows.Scan(&command, &state, &count, &time)
		if err != nil {
			return err
		}
		realState := deriveThreadState(command, state)
		stateCounts[realState] += count
		stateTime[realState] += time
	}

	for state, count := range stateCounts {
		ch <- prometheus.MustNewConstMetric(processlistCountDesc, prometheus.GaugeValue, float64(count), state)
	}
	for state, time := range stateTime {
		ch <- prometheus.MustNewConstMetric(processlistTimeDesc, prometheus.GaugeValue, float64(time), state)
	}

	return nil
}

func deriveThreadState(command string, state string) string {
	var normCmd = strings.Replace(strings.ToLower(command), "_", " ", -1)
	var normState = strings.Replace(strings.ToLower(state), "_", " ", -1)
	// check if it's already a valid state
	_, knownState := threadStateCounterMap[normState]
	if knownState {
		return normState
	}
	// check if plain mapping applies
	mappedState, canMap := threadStateMapping[normState]
	if canMap {
		return mappedState
	}
	// check special waiting for XYZ lock
	if strings.Contains(normState, "waiting for") && strings.Contains(normState, "lock") {
		return "waiting for lock"
	}
	if normCmd == "sleep" && normState == "" {
		return "idle"
	}
	if normCmd == "query" {
		return "executing"
	}
	if normCmd == "binlog dump" {
		return "replication master"
	}
	return "other"
}
