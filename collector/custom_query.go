// Scrape custom queries

package collector

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/yaml.v2"
)

var (
	userQueriesPath = flag.String(
		"queries-file-name", "/usr/local/percona/pmm-client/queries-mysqld.yml", // Default path.
		"Path to custom queries file.",
	)
)

// ColumnUsage should be one of several enum values which describe how a
// queried row is to be converted to a Prometheus metric.
type ColumnUsage int

const (
	discard      ColumnUsage = iota // Ignore this column.
	label                           // Use this column as a label.
	counter                         // Use this column as a counter.
	gauge                           // Use this column as a gauge.
	mappedMetric                    // Use this column with the supplied mapping of text values.
	duration                        // This column should be interpreted as a text duration.
)

// ColumnMapping is the user-friendly representation of a prometheus descriptor map.
type ColumnMapping struct {
	usage       ColumnUsage        `yaml:"usage"`
	description string             `yaml:"description"`
	mapping     map[string]float64 `yaml:"metric_mapping"`
}

// MetricMap stores the prometheus metric description which a given column will
// be mapped to by the collector.
type MetricMap struct {
	discard    bool                              // Should metric be discarded during mapping?
	vtype      prometheus.ValueType              // Prometheus valuetype.
	desc       *prometheus.Desc                  // Prometheus descriptor.
	conversion func(interface{}) (float64, bool) // Conversion function to turn MySQL result into float64.
}

// MetricMapNamespace groups metric maps under a shared set of labels.
type MetricMapNamespace struct {
	labels         []string             // Label names for this namespace.
	columnMappings map[string]MetricMap // Column mappings in this namespace.
}

// CustomQuery - contains MySQL query parsed from YAML file.
type CustomQuery struct {
	mappingMtx      sync.RWMutex
	customMetricMap map[string]MetricMapNamespace
	customQueryMap  map[string]string
}

// ScrapeCustomQuery colects the metrics from custom queries.
type ScrapeCustomQuery struct{}

// Name of the Scraper.
func (scq ScrapeCustomQuery) Name() string {
	return "custom_query"
}

// Help returns additional information about Scraper.
func (scq ScrapeCustomQuery) Help() string {
	return "Collect the metrics from custom queries."
}

// Version of MySQL from which scraper is available.
func (scq ScrapeCustomQuery) Version() float64 {
	return 5.1
}

// Scrape collects data.
func (scq ScrapeCustomQuery) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	cq := CustomQuery{
		customMetricMap: make(map[string]MetricMapNamespace),
		customQueryMap:  make(map[string]string),
	}
	userQueriesData, err := ioutil.ReadFile(*userQueriesPath)
	if err != nil {
		return fmt.Errorf("failed to open custom queries:%s", err.Error())
	}

	cq.mappingMtx.Lock()
	err = addQueries(userQueriesData, cq.customMetricMap, cq.customQueryMap)
	cq.mappingMtx.Unlock()
	if err != nil {
		return fmt.Errorf("failed to add custom queries:%s", err)
	}

	cq.mappingMtx.RLock()
	defer cq.mappingMtx.RUnlock()
	errMap := queryNamespaceMappings(ctx, ch, db, cq.customMetricMap, cq.customQueryMap)
	if len(errMap) > 0 {
		errs := make([]string, 0, len(errMap))
		for k, v := range errMap {
			errs = append(errs, fmt.Sprintf("%s:%s", k, v.Error()))
		}
		return errors.New(strings.Join(errs, ":"))
	}
	return nil
}

// UnmarshalYAML implements the yaml.Unmarshaller interface.
func (cu *ColumnUsage) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return err
	}

	columnUsage, err := stringToColumnUsage(value)
	if err != nil {
		return err
	}

	*cu = columnUsage
	return nil
}

// UnmarshalYAML implements yaml.Unmarshaller.
func (cm *ColumnMapping) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain ColumnMapping
	return unmarshal((*plain)(cm))
}

// addQueries metricMap and customQueryMap to contain the new queries.
// Added queries do not respect version requirements, because it is assumed that
// the user knows what they are doing with their version of mysql.
func addQueries(content []byte, exporterMap map[string]MetricMapNamespace, customQueryMap map[string]string) error {
	var extra map[string]interface{}
	err := yaml.Unmarshal(content, &extra)
	if err != nil {
		return err
	}
	// Stores the loaded map representation.
	metricMaps := make(map[string]map[string]ColumnMapping)
	for metric, specs := range extra {
		log.Debugln("New user metric namespace from YAML:", metric)
		specMap, ok := specs.(map[interface{}]interface{})
		if !ok {
			return fmt.Errorf("incorrect yaml format for %+v", specs)
		}
		for key, value := range specMap {
			switch key.(string) {
			case "query":
				query := value.(string)
				customQueryMap[metric] = query

			case "metrics":
				for _, c := range value.([]interface{}) {
					column := c.(map[interface{}]interface{})
					for n, a := range column {
						var columnMapping ColumnMapping
						// Fetch the metric map we want to work on.
						metricMap, ok := metricMaps[metric]
						if !ok {
							// Namespace for metric not found - add it.
							metricMap = make(map[string]ColumnMapping)
							metricMaps[metric] = metricMap
						}

						// Get name.
						name := n.(string)
						for attrKey, attrVal := range a.(map[interface{}]interface{}) {
							switch attrKey.(string) {
							case "usage":
								usage, err := stringToColumnUsage(attrVal.(string))
								if err != nil {
									return err
								}
								columnMapping.usage = usage
							case "description":
								columnMapping.description = attrVal.(string)
							}
						}

						columnMapping.mapping = nil
						metricMap[name] = columnMapping
					}
				}
			}
		}
	}

	// Convert the loaded metric map into exporter representation.
	// Merge the two maps (which are now quite flatteend).
	makeDescMap(metricMaps, exporterMap)
	return nil
}

// Turn the MetricMap column mapping into a prometheus descriptor mapping.
func makeDescMap(metricMaps map[string]map[string]ColumnMapping, exporterMap map[string]MetricMapNamespace) {
	var metricMap = make(map[string]MetricMapNamespace)
	for namespace, mappings := range metricMaps {
		thisMap := make(map[string]MetricMap)
		// Get the constant labels.
		var constLabels []string
		for columnName, columnMapping := range mappings {
			if columnMapping.usage == label {
				constLabels = append(constLabels, columnName)
			}
		}

		for columnName, columnMapping := range mappings {
			// Determine how to convert the column based on its usage.
			switch columnMapping.usage {
			case discard, label:
				thisMap[columnName] = MetricMap{
					discard: true,
					conversion: func(_ interface{}) (float64, bool) {
						return math.NaN(), true
					},
				}
			case counter:
				thisMap[columnName] = MetricMap{
					vtype: prometheus.CounterValue,
					desc: prometheus.NewDesc(fmt.Sprintf("%s_%s", namespace, columnName),
						columnMapping.description, constLabels, nil),
					conversion: func(in interface{}) (float64, bool) {
						return dbToFloat64(in)
					},
				}
			case gauge:
				thisMap[columnName] = MetricMap{
					vtype: prometheus.GaugeValue,
					desc: prometheus.NewDesc(fmt.Sprintf("%s_%s", namespace, columnName),
						columnMapping.description, constLabels, nil),
					conversion: func(in interface{}) (float64, bool) {
						return dbToFloat64(in)
					},
				}
			case mappedMetric:
				thisMap[columnName] = MetricMap{
					vtype: prometheus.GaugeValue,
					desc: prometheus.NewDesc(fmt.Sprintf("%s_%s", namespace, columnName),
						columnMapping.description, constLabels, nil),
					conversion: func(in interface{}) (float64, bool) {
						text, ok := in.(string)
						if !ok {
							return math.NaN(), false
						}

						val, ok := columnMapping.mapping[text]
						if !ok {
							return math.NaN(), false
						}
						return val, true
					},
				}
			case duration:
				thisMap[columnName] = MetricMap{
					vtype: prometheus.GaugeValue,
					desc: prometheus.NewDesc(fmt.Sprintf("%s_%s_milliseconds", namespace, columnName),
						columnMapping.description, constLabels, nil),
					conversion: func(in interface{}) (float64, bool) {
						var durationString string
						switch t := in.(type) {
						case []byte:
							durationString = string(t)
						case string:
							durationString = t
						default:
							log.Errorln("DURATION conversion metric was not a string")
							return math.NaN(), false
						}

						if durationString == "-1" {
							return math.NaN(), false
						}

						d, err := time.ParseDuration(durationString)
						if err != nil {
							log.Errorln("Failed converting result to metric:", columnName, in, err)
							return math.NaN(), false
						}
						return float64(d / time.Millisecond), true
					},
				}
			}
		}

		metricMap[namespace] = MetricMapNamespace{constLabels, thisMap}
		exporterMap[namespace] = MetricMapNamespace{constLabels, thisMap}
	}
}

// stringToColumnUsage converts a string to the corresponding ColumnUsage.
func stringToColumnUsage(s string) (ColumnUsage, error) {
	switch s {
	case "DISCARD":
		return discard, nil
	case "LABEL":
		return label, nil
	case "COUNTER":
		return counter, nil
	case "GAUGE":
		return gauge, nil
	case "MAPPEDMETRIC":
		return mappedMetric, nil
	case "DURATION":
		return duration, nil
	default:
		return 0, fmt.Errorf("wrong ColumnUsage given : %s", s)
	}
}

// Convert "database/sql value" types to float64s for Prometheus consumption.
// Null types are mapped to NaN. string and []byte types are mapped as NaN and !ok.
func dbToFloat64(t interface{}) (float64, bool) {
	switch v := t.(type) {
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case time.Time:
		return float64(v.UnixNano()) / float64(time.Second), true
	case []byte:
		// Try and convert to string and then parse to a float64
		strV := string(v)
		result, err := strconv.ParseFloat(strV, 64)
		if err != nil {
			log.Warnln("Could not parse []byte:", err)
			return math.NaN(), false
		}
		return result, true
	case string:
		result, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Warnln("Could not parse string:", err)
			return math.NaN(), false
		}
		return result, true
	case nil:
		return math.NaN(), true
	default:
		return math.NaN(), false
	}
}

// Convert "database/sql value" to string for Prometheus labels. Null types are mapped to empty strings.
func dbToString(t interface{}) (string, bool) {
	switch v := t.(type) {
	case int64:
		return strconv.FormatInt(v, 10), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 32), true
	case time.Time:
		return fmt.Sprintf("%v", v.Unix()), true
	case nil:
		return "", true
	case []byte:
		return string(v), true
	case string:
		return v, true
	default:
		return "", false
	}
}

// Query within a namespace mapping and emit metrics. Returns fatal error if
// the scrape fails, and a slice of errors if they were non-fatal.
func queryNamespaceMapping(ctx context.Context, ch chan<- prometheus.Metric,
	db *sql.DB, namespace string, mapping MetricMapNamespace,
	customQueries map[string]string) ([]error, error) {
	// Check for a query override for this namespace.
	query, found := customQueries[namespace]

	if !found {
		return nil, fmt.Errorf("query not found for namespace: %s", namespace)
	}

	// Was this query disabled (i.e. nothing sensible can be queried on cu version of MySQL?
	if query == "" {
		// Return success (no pertinent data).
		return nil, nil
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error running query on database: %s, %s", namespace, err)
	}
	defer rows.Close()
	columnNames, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("error retrieving column list for:  %s, %s", namespace, err)
	}

	// Make a lookup map for the column indices.
	var columnIdx = make(map[string]int, len(columnNames))
	for i, n := range columnNames {
		columnIdx[n] = i
	}

	var columnData = make([]interface{}, len(columnNames))
	var scanArgs = make([]interface{}, len(columnNames))
	for i := range columnData {
		scanArgs[i] = &columnData[i]
	}

	nonfatalErrors := []error{}
	for rows.Next() {
		err = rows.Scan(scanArgs...)
		if err != nil {
			return nil, fmt.Errorf("error retrieving rows: %s, %s", namespace, err)
		}

		// Get the label values for this row
		var labels = make([]string, len(mapping.labels))
		var ok bool
		for idx, columnName := range mapping.labels {
			labels[idx], ok = dbToString(columnData[columnIdx[columnName]])
			if !ok {
				log.Infoln("converted NULL to an empty string")
			}
		}

		// Loop over column names, and match to scan data. Unknown columns
		// will be filled with an untyped metric number *if* they can be
		// converted to float64s. NULLs are allowed and treated as NaN.
		for idx, columnName := range columnNames {
			if metricMapping, ok := mapping.columnMappings[columnName]; ok {

				if metricMapping.discard {
					continue
				}

				value, ok := dbToFloat64(columnData[idx])
				if !ok {
					e := fmt.Errorf("unexpected error parsing column: %s, %s, %s",
						namespace, columnName, columnData[idx])
					nonfatalErrors = append(nonfatalErrors, e)
					continue
				}

				// Generate the metric.
				ch <- prometheus.MustNewConstMetric(metricMapping.desc, metricMapping.vtype, value, labels...)
			} else {
				// Unknown metric. Report as untyped if scan to float64 works, else note an error too.
				metricLabel := fmt.Sprintf("%s_%s", namespace, columnName)
				desc := prometheus.NewDesc(metricLabel, fmt.Sprintf("Unknown metric from %s", namespace), mapping.labels, nil)
				// Its not an error to fail here, since the values are unexpected anyway.
				value, ok := dbToFloat64(columnData[idx])
				if !ok {
					nonfatalErrors = append(nonfatalErrors,
						fmt.Errorf("unparseable column type - discarding: %s, %s, %s", namespace, columnName, err))
					continue
				}
				ch <- prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, value, labels...)
			}
		}
	}

	return nonfatalErrors, nil
}

// Iterate through all the namespace mappings in the exporter and run their queries.
func queryNamespaceMappings(ctx context.Context, ch chan<- prometheus.Metric,
	db *sql.DB, metricMap map[string]MetricMapNamespace, customQueries map[string]string) map[string]error {
	// Return a map of namespace -> errors.
	namespaceErrors := make(map[string]error)
	for namespace, mapping := range metricMap {
		nonFatalErrors, err := queryNamespaceMapping(ctx, ch, db, namespace, mapping, customQueries)
		// Serious error - a namespace disappeared.
		if err != nil {
			namespaceErrors[namespace] = err
		}
		// Non-serious errors - likely version or parsing problems.
		if len(nonFatalErrors) > 0 {
			for _, err := range nonFatalErrors {
				log.Infoln(err.Error())
			}
		}
	}

	return namespaceErrors
}
