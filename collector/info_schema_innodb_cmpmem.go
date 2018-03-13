// Scrape `information_schema.innodb_cmpmem`.

package collector

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const innodbCmpMemQuery = `
                SELECT
                  page_size, buffer_pool_instance, pages_used, pages_free, relocation_ops, relocation_time 
                  FROM information_schema.INNODB_CMPMEM
                `

var (
	// Map known innodb_cmp values to types. Unknown types will be mapped as
	// untyped.
	informationSchemaInnodbCmpMemTypes = map[string]struct {
		vtype prometheus.ValueType
		desc  *prometheus.Desc
	}{
		"page_size": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_page_size"),
				"Block size in bytes.",
				[]string{"page_size"}, nil)},
		"buffer_pool_instance": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_buffer_pool_instance"),
				"A unique identifier for the buffer pool instance.",
				[]string{"page_size"}, nil)},
		"pages_used": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_pages_used"),
				"Number of blocks of the size PAGE_SIZE that are currently in use.",
				[]string{"page_size"}, nil)},
		"pages_free": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_pages_free"),
				"Number of blocks of the size PAGE_SIZE that are currently available for allocation.",
				[]string{"page_size"}, nil)},
		"relocation_ops": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_relocation_ops"),
				"Number of times a block of the size PAGE_SIZE has been relocated.",
				[]string{"page_size"}, nil)},
		"relocation_time": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_relocation_time"),
				"Total time in microseconds spent in relocating blocks of the size PAGE_SIZE.",
				[]string{"page_size"}, nil)},
	}
)

// ScrapeClientStat collects from `information_schema.innodb_cmpmem`.
type ScrapeInnodbCmpMem struct{}

// Name of the Scraper.
func (ScrapeInnodbCmpMem) Name() string {
	return "info_schema.innodb_cmpmem"
}

// Help returns additional information about Scraper.
func (ScrapeInnodbCmpMem) Help() string {
	return "Please set next variables SET GLOBAL innodb_file_per_table=1;SET GLOBAL innodb_file_format=Barracuda;"
}

// Version of MySQL from which scraper is available.
func (ScrapeInnodbCmpMem) Version() float64 {
	return 5.5
}

// Scrape collects data.
func (ScrapeInnodbCmpMem) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
	informationSchemaInnodbCmpMemRows, err := db.Query(innodbCmpMemQuery)
	if err != nil {
		log.Debugln("INNODB_CMPMEM stats are not available.")
		return err
	}
	defer informationSchemaInnodbCmpMemRows.Close()

	// The client column is assumed to be column[0], while all other data is assumed to be coerceable to float64.
	// Because of the client column, clientStatData[0] maps to columnNames[1] when reading off the metrics
	// (because clientStatScanArgs is mapped as [ &client, &clientData[0], &clientData[1] ... &clientdata[n] ]
	// To map metrics to names therefore we always range over columnNames[1:]
	columnNames, err := informationSchemaInnodbCmpMemRows.Columns()
	if err != nil {
		log.Debugln("INNODB_CMPMEM stats are not available.")
		return err
	}

	var (
		client             string                                // Holds the client name, which should be in column 0.
		clientStatData     = make([]float64, len(columnNames)-1) // 1 less because of the client column.
		clientStatScanArgs = make([]interface{}, len(columnNames))
	)

	clientStatScanArgs[0] = &client
	for i := range clientStatData {
		clientStatScanArgs[i+1] = &clientStatData[i]
	}

	for informationSchemaInnodbCmpMemRows.Next() {
		if err := informationSchemaInnodbCmpMemRows.Scan(clientStatScanArgs...); err != nil {
			return err
		}

		// Loop over column names, and match to scan data. Unknown columns
		// will be filled with an untyped metric number. We assume other then
		// cient, that we'll only get numbers.
		for idx, columnName := range columnNames[1:] {
			if metricType, ok := informationSchemaInnodbCmpMemTypes[columnName]; ok {
				ch <- prometheus.MustNewConstMetric(metricType.desc, metricType.vtype, float64(clientStatData[idx]), client)
			} else {
				// Unknown metric. Report as untyped.
				desc := prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, fmt.Sprintf("innodb_cmpmem_%s", strings.ToLower(columnName))), fmt.Sprintf("Unsupported metric from column %s", columnName), []string{"page_size"}, nil)
				ch <- prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, float64(clientStatData[idx]), client)
			}
		}
	}
	return nil
}
