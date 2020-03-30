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

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const perfReplicationGroupMemebersQuery = `
	SELECT CHANNEL_NAME,MEMBER_ID,MEMBER_HOST,MEMBER_PORT,MEMBER_STATE,MEMBER_ROLE,MEMBER_VERSION
	  FROM performance_schema.replication_group_members
	`

// Metric descriptors.
var (
	performanceSchemaReplicationGroupMembersMemberDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "replication_group_member"),
		"Information about the replication group member: "+
			"channel_name, member_id, member_host, member_port, member_state, member_role, member_version. ",

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

		[]string{"channel_name", "member_id", "member_host", "member_port", "member_state", "member_role", "member_version"}, nil,
	)
)

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
	return 8.0
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfReplicationGroupMembers) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	perfReplicationGroupMembersRows, err := db.QueryContext(ctx, perfReplicationGroupMemebersQuery)
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
		if err := perfReplicationGroupMembersRows.Scan(
			&channelName, &memberId, &memberHost, &memberPort, &memberState, &memberRole, &memberVersion,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationGroupMembersMemberDesc, prometheus.GaugeValue, 1,
			channelName, memberId, memberHost, memberPort, memberState, memberRole, memberVersion,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfReplicationGroupMembers{}
