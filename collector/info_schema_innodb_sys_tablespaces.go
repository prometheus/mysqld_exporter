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

// Scrape `information_schema.innodb_sys_tablespaces`.

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
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

// Metric descriptors.
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
type ScrapeInfoSchemaInnodbTablespaces struct{}

// Name of the Scraper. Should be unique.
func (ScrapeInfoSchemaInnodbTablespaces) Name() string {
	return informationSchema + ".innodb_tablespaces"
}

// Help describes the role of the Scraper.
func (ScrapeInfoSchemaInnodbTablespaces) Help() string {
	return "Collect metrics from information_schema.innodb_sys_tablespaces"
}

// Version of MySQL from which scraper is available.
func (ScrapeInfoSchemaInnodbTablespaces) Version() float64 {
	return 5.7
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeInfoSchemaInnodbTablespaces) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	tablespacesRows, err := db.QueryContext(ctx, innodbTablespacesQuery)
	if err != nil {
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

// check interface
var _ Scraper = ScrapeInfoSchemaInnodbTablespaces{}
