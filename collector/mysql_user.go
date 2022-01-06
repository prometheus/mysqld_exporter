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

// Scrape `mysql.user`.

package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const mysqlUserQuery = `
		  SELECT
		    user,
		    host,
		    Select_priv,
		    Insert_priv,
		    Update_priv,
		    Delete_priv,
		    Create_priv,
		    Drop_priv,
		    Reload_priv,
		    Shutdown_priv,
		    Process_priv,
		    File_priv,
		    Grant_priv,
		    References_priv,
		    Index_priv,
		    Alter_priv,
		    Show_db_priv,
		    Super_priv,
		    Create_tmp_table_priv,
		    Lock_tables_priv,
		    Execute_priv,
		    Repl_slave_priv,
		    Repl_client_priv,
		    Create_view_priv,
		    Show_view_priv,
		    Create_routine_priv,
		    Alter_routine_priv,
		    Create_user_priv,
		    Event_priv,
		    Trigger_priv,
		    Create_tablespace_priv,
		    max_questions,
		    max_updates,
		    max_connections,
		    max_user_connections
		  FROM mysql.user
		`

// Tunable flags.
var (
	userPrivilegesFlag = kingpin.Flag(
		"collect.mysql.user.privileges",
		"Enable collecting user privileges from mysql.user",
	).Default("false").Bool()
)

var (
	labelNames = []string{"mysql_user", "hostmask"}
)

// Metric descriptors.
var (
	userMaxQuestionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "max_questions"),
		"The number of max_questions by user.",
		labelNames, nil)
	userMaxUpdatesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "max_updates"),
		"The number of max_updates by user.",
		labelNames, nil)
	userMaxConnectionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "max_connections"),
		"The number of max_connections by user.",
		labelNames, nil)
	userMaxUserConnectionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "max_user_connections"),
		"The number of max_user_connections by user.",
		labelNames, nil)
)

// ScrapeUser collects from `information_schema.processlist`.
type ScrapeUser struct{}

// Name of the Scraper. Should be unique.
func (ScrapeUser) Name() string {
	return mysql + ".user"
}

// Help describes the role of the Scraper.
func (ScrapeUser) Help() string {
	return "Collect data from mysql.user"
}

// Version of MySQL from which scraper is available.
func (ScrapeUser) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeUser) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	var (
		userRows *sql.Rows
		err      error
	)
	userQuery := fmt.Sprint(mysqlUserQuery)
	userRows, err = db.QueryContext(ctx, userQuery)
	if err != nil {
		return err
	}
	defer userRows.Close()

	var (
		user                   string
		host                   string
		Select_priv            string
		Insert_priv            string
		Update_priv            string
		Delete_priv            string
		Create_priv            string
		Drop_priv              string
		Reload_priv            string
		Shutdown_priv          string
		Process_priv           string
		File_priv              string
		Grant_priv             string
		References_priv        string
		Index_priv             string
		Alter_priv             string
		Show_db_priv           string
		Super_priv             string
		Create_tmp_table_priv  string
		Lock_tables_priv       string
		Execute_priv           string
		Repl_slave_priv        string
		Repl_client_priv       string
		Create_view_priv       string
		Show_view_priv         string
		Create_routine_priv    string
		Alter_routine_priv     string
		Create_user_priv       string
		Event_priv             string
		Trigger_priv           string
		Create_tablespace_priv string
		max_questions          uint32
		max_updates            uint32
		max_connections        uint32
		max_user_connections   uint32
	)

	for userRows.Next() {
		err = userRows.Scan(
			&user,
			&host,
			&Select_priv,
			&Insert_priv,
			&Update_priv,
			&Delete_priv,
			&Create_priv,
			&Drop_priv,
			&Reload_priv,
			&Shutdown_priv,
			&Process_priv,
			&File_priv,
			&Grant_priv,
			&References_priv,
			&Index_priv,
			&Alter_priv,
			&Show_db_priv,
			&Super_priv,
			&Create_tmp_table_priv,
			&Lock_tables_priv,
			&Execute_priv,
			&Repl_slave_priv,
			&Repl_client_priv,
			&Create_view_priv,
			&Show_view_priv,
			&Create_routine_priv,
			&Alter_routine_priv,
			&Create_user_priv,
			&Event_priv,
			&Trigger_priv,
			&Create_tablespace_priv,
			&max_questions,
			&max_updates,
			&max_connections,
			&max_user_connections,
		)

		if err != nil {
			return err
		}

		if *userPrivilegesFlag {
			userCols, err := userRows.Columns()
			if err != nil {
				return err
			}

			scanArgs := make([]interface{}, len(userCols))
			for i := range scanArgs {
				scanArgs[i] = &sql.RawBytes{}
			}

			if err := userRows.Scan(scanArgs...); err != nil {
				return err
			}

			for i, col := range userCols {
				if value, ok := parsePrivilege(*scanArgs[i].(*sql.RawBytes)); ok { // Silently skip unparsable values.
					ch <- prometheus.MustNewConstMetric(
						prometheus.NewDesc(
							prometheus.BuildFQName(namespace, mysql, strings.ToLower(col)),
							col+" by user.",
							labelNames,
							nil,
						),
						prometheus.GaugeValue,
						value,
						user, host,
					)
				}
			}
		}

		ch <- prometheus.MustNewConstMetric(userMaxQuestionsDesc, prometheus.GaugeValue, float64(max_questions), user, host)
		ch <- prometheus.MustNewConstMetric(userMaxUpdatesDesc, prometheus.GaugeValue, float64(max_updates), user, host)
		ch <- prometheus.MustNewConstMetric(userMaxConnectionsDesc, prometheus.GaugeValue, float64(max_connections), user, host)
		ch <- prometheus.MustNewConstMetric(userMaxUserConnectionsDesc, prometheus.GaugeValue, float64(max_user_connections), user, host)
	}

	return nil
}
