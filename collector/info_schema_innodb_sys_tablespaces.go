// Scrape `information_schema.innodb_sys_tablespaces`.

package collector

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const innodbTablespacesQuery = `
	SELECT
	    SPACE,
	    NAME,
	    ifnull(FILE_FORMAT, 'NONE') as FILE_FORMAT,
	    ifnull(ROW_FORMAT, 'NONE') as ROW_FORMAT,
	    ifnull(SPACE_TYPE, 'NONE') as SPACE_TYPE,
	    FILE_SIZE,
	    ALLOCATED_SIZE
	  FROM information_schema.innodb_sys_tablespaces
	`

var (
	infoSchemaInnodbTablesspaceInfoDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_tablespace_space_info"),
		"The Tablespace information and Space ID.",
		[]string{"tablespace_name", "file_format", "row_format", "space_type"}, nil,
	)
	infoSchemaInnodbTablesspaceFileSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_tablespace_file_size_bytes"),
		"The apparent size of the file, which represents the maximum size of the file, uncompressed.",
		[]string{"tablespace_name"}, nil,
	)
	infoSchemaInnodbTablesspaceAllocatedSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_tablespace_allocated_size_bytes"),
		"The actual size of the file, which is the amount of space allocated on disk.",
		[]string{"tablespace_name"}, nil,
	)
)

// ScrapeInfoSchemaInnodbTablespaces collects from `information_schema.innodb_sys_tablespaces`.
func ScrapeInfoSchemaInnodbTablespaces(db *sql.DB, ch chan<- prometheus.Metric) error {
	tablespacesRows, err := db.Query(innodbTablespacesQuery)
	// Soft fail if SPACE_TYPE is not in the field list as this collector is useful and working with
	// MySQL 5.7. See https://github.com/prometheus/mysqld_exporter/issues/118
	if err != nil && err.Error() == "Error 1054: Unknown column 'SPACE_TYPE' in 'field list'" {
		log.Debugln("Ignoring -collect.info_schema.innodb_tablespaces for MySQL prior 5.7.")
		return nil
	} else if err != nil {
		return err
	}
	defer tablespacesRows.Close()

	var (
		tableSpace    uint32
		tableName     string
		fileFormat    string
		rowFormat     string
		spaceType     string
		fileSize      uint64
		allocatedSize uint64
	)

	for tablespacesRows.Next() {
		err = tablespacesRows.Scan(
			&tableSpace,
			&tableName,
			&fileFormat,
			&rowFormat,
			&spaceType,
			&fileSize,
			&allocatedSize,
		)
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			infoSchemaInnodbTablesspaceInfoDesc, prometheus.GaugeValue, float64(tableSpace),
			tableName, fileFormat, rowFormat, spaceType,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaInnodbTablesspaceFileSizeDesc, prometheus.GaugeValue, float64(fileSize),
			tableName,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaInnodbTablesspaceAllocatedSizeDesc, prometheus.GaugeValue, float64(allocatedSize),
			tableName,
		)
	}

	return nil
}
