// Scrape `information_schema.INNODB_CMPMEM`.

package collector

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

const innodbCmpMemQuery = `
                SELECT
                  page_size, buffer_pool_instance, pages_used, pages_free, relocation_ops, relocation_time 
                  FROM information_schema.innodb_cmpmem
                `

var (
	infoSchemaInnodbCmpMemPagesRead = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_pages_used_total"),
		"Number of blocks of the size PAGE_SIZE that are currently in use.",
		[]string{"page_size", "buffer"}, nil,
	)
	infoSchemaInnodbCmpMemPagesFree = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_pages_free_total"),
		"Number of blocks of the size PAGE_SIZE that are currently available for allocation.",
		[]string{"page_size", "buffer"}, nil,
	)
	infoSchemaInnodbCmpMemRelocationOps = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_relocation_ops_total"),
		"Number of times a block of the size PAGE_SIZE has been relocated.",
		[]string{"page_size", "buffer"}, nil,
	)
	infoSchemaInnodbCmpMemRelocationTime = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_relocation_time_seconds_total"),
		"Total time in seconds spent in relocating blocks.",
		[]string{"page_size", "buffer"}, nil,
	)
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

		for idx, columnName := range columnNames[2:] {
			switch columnName {
			case "pages_used":
				ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpMemPagesRead, prometheus.CounterValue, float64(clientCmpMemData[idx]), page_size, buffer)
			case "pages_free":
				ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpMemPagesFree, prometheus.CounterValue, float64(clientCmpMemData[idx]), page_size, buffer)
			case "relocation_ops":
				ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpMemRelocationOps, prometheus.CounterValue, float64(clientCmpMemData[idx]), page_size, buffer)
			case "relocation_time":
				ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpMemRelocationTime, prometheus.CounterValue, float64(clientCmpMemData[idx]/1000), page_size, buffer)
			}
		}
	}
	return nil
}
