// Copyright 2020 The Prometheus Authors
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
	"context"
	"database/sql"
	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
)

const perfReplicationGroupMembersQuery = `
  SELECT * FROM performance_schema.replication_group_members
	`

// ScrapeReplicationGroupMembers collects from `performance_schema.replication_group_members`.
type ScrapePerfReplicationGroupMembers struct{}

// Name of the Scraper. Should be unique.
func (ScrapePerfReplicationGroupMembers) Name() string {
	return performanceSchema + ".replication_group_members"
}

// Help describes the role of the Scraper.
func (ScrapePerfReplicationGroupMembers) Help() string {
	return "Collect metrics from performance_schema.replication_group_members"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfReplicationGroupMembers) Version() float64 {
	return 5.7
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfReplicationGroupMembers) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	perfReplicationGroupMembersRows, err := db.QueryContext(ctx, perfReplicationGroupMembersQuery)
	if err != nil {
		return err
	}
	defer perfReplicationGroupMembersRows.Close()

	var columnNames []string
	if columnNames, err = perfReplicationGroupMembersRows.Columns(); err != nil {
		return err
	}

	var scanArgs = make([]interface{}, len(columnNames))
	for i := range scanArgs {
		scanArgs[i] = &sql.RawBytes{}
	}

	for perfReplicationGroupMembersRows.Next() {
		if err := perfReplicationGroupMembersRows.Scan(scanArgs...); err != nil {
			return err
		}

		var labelNames = make([]string, len(columnNames))
		var values = make([]string, len(columnNames))
		for i, columnName := range columnNames {
			labelNames[i] = strings.ToLower(columnName)
			values[i] = string(*scanArgs[i].(*sql.RawBytes))
		}

		var performanceSchemaReplicationGroupMembersMemberDesc = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, performanceSchema, "replication_group_member_info"),
			"Information about the replication group member: "+
				"channel_name, member_id, member_host, member_port, member_state. "+
				"(member_role and member_version where available)",
			labelNames, nil,
		)

		ch <- prometheus.MustNewConstMetric(performanceSchemaReplicationGroupMembersMemberDesc,
			prometheus.GaugeValue, 1, values...)
	}
	return nil

}

// check interface
var _ Scraper = ScrapePerfReplicationGroupMembers{}
