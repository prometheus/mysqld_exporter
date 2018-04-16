// Scrape `information_schema.INNODB_CMPMEM`.

package collector

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	//	"github.com/prometheus/common/log"
)

const innodbCmpMemQuery = `SELECT * FROM information_schema.INNODB_CMPMEM`

var (
	// Map known client-statistics values to types. Unknown types will be mapped as
	// untyped.
	informationSchemaInnodbCmpMemTypes = map[string]struct {
		vtype prometheus.ValueType
		desc  *prometheus.Desc
	}{
		"pages_used": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_pages_used_total"),
				"Number of blocks of the size PAGE_SIZE that are currently in use.",
				[]string{"page_size", "buffer"}, nil)},
		"pages_free": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_pages_free_total"),
				"Number of blocks of the size PAGE_SIZE that are currently available for allocation.",
				[]string{"page_size", "buffer"}, nil)},
		"relocation_ops": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_relocation_ops_total"),
				"Number of times a block of the size PAGE_SIZE has been relocated.",
				[]string{"page_size", "buffer"}, nil)},
		"relocation_time": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_relocation_time_seconds_total"),
				"Total time in seconds spent in relocating blocks.",
				[]string{"page_size", "buffer"}, nil)},
	}
)

// ScrapeInnodbCmp collects from `information_schema.innodb_cmp`.
func ScrapeInnodbCmpMem(db *sql.DB, ch chan<- prometheus.Metric) error {

	informationSchemaInnodbCmpMemRows, err := db.Query(innodbCmpMemQuery)
	if err != nil {
		return err
	}
	defer informationSchemaInnodbCmpMemRows.Close()

	// The client column is assumed to be column[0], while all other data is assumed to be coerceable to float64.
	// Because of the client column, clientStatData[0] maps to columnNames[1] when reading off the metrics
	// (because clientStatScanArgs is mapped as [ &client, &clientData[0], &clientData[1] ... &clientdata[n] ]
	// To map metrics to names therefore we always range over columnNames[1:]
	columnNames, err := informationSchemaInnodbCmpMemRows.Columns()
	if err != nil {
		return err
	}

	var (
		page_size            string                                // Holds the page size, which should be in column 0.
		buffer               string                                // Holds the buffer number, which should be in column 1.
		clientCmpMemData     = make([]float64, len(columnNames)-2) // 2 less because of the client and buffer columns.
		clientCmpMemScanArgs = make([]interface{}, len(columnNames))
	)

	clientCmpMemScanArgs[0] = &page_size
	clientCmpMemScanArgs[1] = &buffer
	for i := range clientCmpMemData {
		clientCmpMemScanArgs[i+2] = &clientCmpMemData[i]
	}
	for informationSchemaInnodbCmpMemRows.Next() {
		if err := informationSchemaInnodbCmpMemRows.Scan(clientCmpMemScanArgs...); err != nil {
			return err
		}

		// Loop over column names, and match to scan data. Unknown columns
		// will be filled with an untyped metric number. We assume other then
		// cient, that we'll only get numbers.
		for idx, columnName := range columnNames[2:] {
			if metricType, ok := informationSchemaInnodbCmpMemTypes[columnName]; ok {
				if columnName == "relocation_time" {
					ch <- prometheus.MustNewConstMetric(metricType.desc, metricType.vtype, float64(clientCmpMemData[idx]/1000), page_size, buffer)
				} else {
					ch <- prometheus.MustNewConstMetric(metricType.desc, metricType.vtype, float64(clientCmpMemData[idx]), page_size, buffer)
				}
			} else {
				// Unknown metric. Report as untyped.
				desc := prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, fmt.Sprintf("innodb_cmpmem_%s", strings.ToLower(columnName))), fmt.Sprintf("Unsupported metric from column %s", columnName), []string{"page_size", "buffer"}, nil)
				ch <- prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, float64(clientCmpMemData[idx]), page_size, buffer)
			}
		}
	}
	return nil
}
