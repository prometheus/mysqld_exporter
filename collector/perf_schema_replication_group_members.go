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
	"github.com/go-kit/kit/log/level"
	MySQL "github.com/go-sql-driver/mysql"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const perfReplicationGroupMembersQueryWithAdditionalCols = `
  SELECT
    CHANNEL_NAME,
    MEMBER_ID,
    MEMBER_HOST,
    MEMBER_PORT,
    MEMBER_STATE,
    MEMBER_ROLE,
    MEMBER_VERSION
  FROM performance_schema.replication_group_members
	`
const perfReplicationGroupMembersQuery = `
  SELECT 
    CHANNEL_NAME,
    MEMBER_ID,
    MEMBER_HOST,
    MEMBER_PORT,
    MEMBER_STATE
  FROM performance_schema.replication_group_members
	`

// Metric descriptors.
var (
	/*	channel_name: Name of the Group Replication channel.
		member_id:  The member server UUID. This has a different value for each member in the group. This also serves as a key because it is unique to each member.
		member_host:  Network address of this member (host name or IP address). Retrieved from the member's hostname variable.
			This is the address which clients connect to, unlike the group_replication_local_address which is used for internal group communication.
		member_port: Port on which the server is listening. Retrieved from the member's port variable.
		member_state: Current state of this member; can be any one of the following:
			ONLINE: The member is in a fully functioning state.
			RECOVERING: The server has joined a group from which it is retrieving data.
			OFFLINE: The group replication plugin is installed but has not been started.
			ERROR: The member has encountered an error, either during applying transactions or during the recovery phase, and is not participating in the group's transactions.
			UNREACHABLE: The failure detection process suspects that this member cannot be contacted, because the group messages have timed out.
		member_role: Role of the member in the group, either PRIMARY or SECONDARY.
		member_version: MySQL version of the member.
	*/
	performanceSchemaReplicationGroupMembersMemberWithAdditionalColsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "replication_group_member"),
		"Information about the replication group member: "+
			"channel_name, member_id, member_host, member_port, member_state, member_role, member_version. ",
		[]string{"channel_name", "member_id", "member_host", "member_port", "member_state", "member_role", "member_version"}, nil,
	)
	performanceSchemaReplicationGroupMembersMemberDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "replication_group_member"),
		"Information about the replication group member: "+
			"channel_name, member_id, member_host, member_port, member_state. ",
		[]string{"channel_name", "member_id", "member_host", "member_port", "member_state"}, nil,
	)
)

var activeQuery string

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
	perfReplicationGroupMembersRows, err := db.QueryContext(ctx, getPerfReplicationGroupMembersQuery(ctx, db, logger))
	if err != nil {
		return err
	}
	defer perfReplicationGroupMembersRows.Close()

	var (
		channelName   string
		memberId      string
		memberHost    string
		memberPort    string
		memberState   string
		memberRole    string
		memberVersion string
	)

	for perfReplicationGroupMembersRows.Next() {
		if getPerfReplicationGroupMembersQuery(ctx, db, logger) == perfReplicationGroupMembersQueryWithAdditionalCols {
			if err := perfReplicationGroupMembersRows.Scan(
				&channelName, &memberId, &memberHost, &memberPort, &memberState, &memberRole, &memberVersion,
			); err != nil {
				return err
			}
			ch <- prometheus.MustNewConstMetric(
				performanceSchemaReplicationGroupMembersMemberWithAdditionalColsDesc, prometheus.GaugeValue, 1,
				channelName, memberId, memberHost, memberPort, memberState, memberRole, memberVersion,
			)
		} else {
			if err := perfReplicationGroupMembersRows.Scan(
				&channelName, &memberId, &memberHost, &memberPort, &memberState,
			); err != nil {
				return err
			}
			ch <- prometheus.MustNewConstMetric(
				performanceSchemaReplicationGroupMembersMemberDesc, prometheus.GaugeValue, 1,
				channelName, memberId, memberHost, memberPort, memberState,
			)
		}

	}
	return nil
}

func getPerfReplicationGroupMembersQuery(ctx context.Context, db *sql.DB, logger log.Logger) string {
	if activeQuery == "" {
		activeQuery = perfReplicationGroupMembersQueryWithAdditionalCols
		_, err := db.QueryContext(ctx, activeQuery)
		if err != nil {
			if mysqlErr, ok := err.(*MySQL.MySQLError); ok { // Now the error number is accessible directly
				// Check for error 1054: Unknown column.
				if mysqlErr.Number == 1054 {
					level.Debug(logger).Log("msg", "Additional columns for performance_schema.replication_group_members are not available.")
					activeQuery = perfReplicationGroupMembersQuery
				}
			}
		}
	}
	return activeQuery
}

// check interface
var _ Scraper = ScrapePerfReplicationGroupMembers{}
