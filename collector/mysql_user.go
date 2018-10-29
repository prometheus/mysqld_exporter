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

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const mysqlUserQuery = `
		  SELECT
		    user,
		    host,
		    IF( Select_priv = 'N', 0, 1 ) AS select_priv,
		    IF( Insert_priv = 'N', 0, 1 ) AS insert_priv,
		    IF( Update_priv = 'N', 0, 1 ) AS update_priv,
		    IF( Delete_priv = 'N', 0, 1 ) AS delete_priv,
		    IF( Create_priv = 'N', 0, 1 ) AS create_priv,
		    IF( Drop_priv = 'N', 0, 1 ) AS drop_priv,
		    IF( Reload_priv = 'N', 0, 1 ) AS reload_priv,
		    IF( Shutdown_priv = 'N', 0, 1 ) AS shutdown_priv,
		    IF( Process_priv = 'N', 0, 1 ) AS process_priv,
		    IF( File_priv = 'N', 0, 1 ) AS file_priv,
		    IF( Grant_priv = 'N', 0, 1 ) AS grant_priv,
		    IF( References_priv = 'N', 0, 1 ) AS references_priv,
		    IF( Index_priv = 'N', 0, 1 ) AS index_priv,
		    IF( Alter_priv = 'N', 0, 1 ) AS alter_priv,
		    IF( Show_db_priv = 'N', 0, 1 ) AS show_db_priv,
		    IF( Super_priv = 'N', 0, 1 ) AS super_priv,
		    IF( Create_tmp_table_priv = 'N', 0, 1 ) AS create_tmp_table_priv,
		    IF( Lock_tables_priv = 'N', 0, 1 ) AS lock_tables_priv,
		    IF( Execute_priv = 'N', 0, 1 ) AS execute_priv,
		    IF( Repl_slave_priv = 'N', 0, 1 ) AS repl_slave_priv,
		    IF( Repl_client_priv = 'N', 0, 1 ) AS repl_client_priv,
		    IF( Create_view_priv = 'N', 0, 1 ) AS create_view_priv,
		    IF( Show_view_priv = 'N', 0, 1 ) AS show_view_priv,
		    IF( Create_routine_priv = 'N', 0, 1 ) AS create_routine_priv,
		    IF( Alter_routine_priv = 'N', 0, 1 ) AS alter_routine_priv,
		    IF( Create_user_priv = 'N', 0, 1 ) AS create_user_priv,
		    IF( Event_priv = 'N', 0, 1 ) AS event_priv,
		    IF( Trigger_priv = 'N', 0, 1 ) AS trigger_priv,
		    IF( Create_tablespace_priv = 'N', 0, 1 ) AS create_tablespace_priv,
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
	userSelectPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "select_priv"),
		"Select_priv by user.",
		labelNames, nil)
	userInsertPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "insert_priv"),
		"Insert_priv by user.",
		labelNames, nil)
	userUpdatePrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "update_priv"),
		"Update_priv by user.",
		labelNames, nil)
	userDeletePrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "delete_priv"),
		"Delete_priv by user.",
		labelNames, nil)
	userCreatePrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "create_priv"),
		"Create_priv by user.",
		labelNames, nil)
	userDropPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "drop_priv"),
		"Drop_priv by user.",
		labelNames, nil)
	userReloadPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "reload_priv"),
		"Reload_priv by user.",
		labelNames, nil)
	userShutdownPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "shutdown_priv"),
		"Shutdown_priv by user.",
		labelNames, nil)
	userProcessPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "process_priv"),
		"Process_priv by user.",
		labelNames, nil)
	userFilePrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "file_priv"),
		"File_priv by user.",
		labelNames, nil)
	userGrantPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "grant_priv"),
		"Grant_priv by user.",
		labelNames, nil)
	userReferencesPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "references_priv"),
		"References_priv by user.",
		labelNames, nil)
	userIndexPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "index_priv"),
		"Index_priv by user.",
		labelNames, nil)
	userAlterPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "alter_priv"),
		"Alter_priv by user.",
		labelNames, nil)
	userShowDbPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "show_db_priv"),
		"Show_db_priv by user.",
		labelNames, nil)
	userSuperPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "super_priv"),
		"Super_priv by user.",
		labelNames, nil)
	userCreateTmpTablePrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "create_tmp_table_priv"),
		"Create_tmp_table_priv by user.",
		labelNames, nil)
	userLockTablesPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "lock_tables_priv"),
		"Lock_tables_priv by user.",
		labelNames, nil)
	userExecutePrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "execute_priv"),
		"Execute_priv by user.",
		labelNames, nil)
	userReplSlavePrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "repl_slave_priv"),
		"Repl_slave_priv by user.",
		labelNames, nil)
	userReplClientPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "repl_client_priv"),
		"Repl_client_priv by user.",
		labelNames, nil)
	userCreateViewPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "create_view_priv"),
		"Create_view_priv by user.",
		labelNames, nil)
	userShowViewPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "show_view_priv"),
		"Show_view_priv by user.",
		labelNames, nil)
	userCreateRoutinePrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "create_routine_priv"),
		"Create_routine_priv by user.",
		labelNames, nil)
	userAlterRoutinePrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "alter_routine_priv"),
		"Alter_routine_priv by user.",
		labelNames, nil)
	userCreateUserPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "create_user_priv"),
		"Create_user_priv by user.",
		labelNames, nil)
	userEventPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "event_priv"),
		"Event_priv by user.",
		labelNames, nil)
	userTriggerPrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "trigger_priv"),
		"Trigger_priv by user.",
		labelNames, nil)
	userCreateTablespacePrivDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "create_tablespace_priv"),
		"Create_tablespace_priv by user.",
		labelNames, nil)
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
func (ScrapeUser) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	userQuery := fmt.Sprint(mysqlUserQuery)
	userRows, err := db.QueryContext(ctx, userQuery)
	if err != nil {
		return err
	}
	defer userRows.Close()

	var (
		user                   string
		host                   string
		select_priv            uint32
		insert_priv            uint32
		update_priv            uint32
		delete_priv            uint32
		create_priv            uint32
		drop_priv              uint32
		reload_priv            uint32
		shutdown_priv          uint32
		process_priv           uint32
		file_priv              uint32
		grant_priv             uint32
		references_priv        uint32
		index_priv             uint32
		alter_priv             uint32
		show_db_priv           uint32
		super_priv             uint32
		create_tmp_table_priv  uint32
		lock_tables_priv       uint32
		execute_priv           uint32
		repl_slave_priv        uint32
		repl_client_priv       uint32
		create_view_priv       uint32
		show_view_priv         uint32
		create_routine_priv    uint32
		alter_routine_priv     uint32
		create_user_priv       uint32
		event_priv             uint32
		trigger_priv           uint32
		create_tablespace_priv uint32
		max_questions          uint32
		max_updates            uint32
		max_connections        uint32
		max_user_connections   uint32
	)

	for userRows.Next() {
		err = userRows.Scan(
			&user,
			&host,
			&select_priv,
			&insert_priv,
			&update_priv,
			&delete_priv,
			&create_priv,
			&drop_priv,
			&reload_priv,
			&shutdown_priv,
			&process_priv,
			&file_priv,
			&grant_priv,
			&references_priv,
			&index_priv,
			&alter_priv,
			&show_db_priv,
			&super_priv,
			&create_tmp_table_priv,
			&lock_tables_priv,
			&execute_priv,
			&repl_slave_priv,
			&repl_client_priv,
			&create_view_priv,
			&show_view_priv,
			&create_routine_priv,
			&alter_routine_priv,
			&create_user_priv,
			&event_priv,
			&trigger_priv,
			&create_tablespace_priv,
			&max_questions,
			&max_updates,
			&max_connections,
			&max_user_connections,
		)

		if err != nil {
			return err
		}

		if *userPrivilegesFlag == true {
			ch <- prometheus.MustNewConstMetric(userSelectPrivDesc, prometheus.GaugeValue, float64(select_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userInsertPrivDesc, prometheus.GaugeValue, float64(insert_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userUpdatePrivDesc, prometheus.GaugeValue, float64(update_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userDeletePrivDesc, prometheus.GaugeValue, float64(delete_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userCreatePrivDesc, prometheus.GaugeValue, float64(create_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userDropPrivDesc, prometheus.GaugeValue, float64(drop_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userReloadPrivDesc, prometheus.GaugeValue, float64(reload_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userShutdownPrivDesc, prometheus.GaugeValue, float64(shutdown_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userProcessPrivDesc, prometheus.GaugeValue, float64(process_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userFilePrivDesc, prometheus.GaugeValue, float64(file_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userGrantPrivDesc, prometheus.GaugeValue, float64(grant_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userReferencesPrivDesc, prometheus.GaugeValue, float64(references_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userIndexPrivDesc, prometheus.GaugeValue, float64(index_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userAlterPrivDesc, prometheus.GaugeValue, float64(alter_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userShowDbPrivDesc, prometheus.GaugeValue, float64(show_db_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userSuperPrivDesc, prometheus.GaugeValue, float64(super_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userCreateTmpTablePrivDesc, prometheus.GaugeValue, float64(create_tmp_table_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userLockTablesPrivDesc, prometheus.GaugeValue, float64(lock_tables_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userExecutePrivDesc, prometheus.GaugeValue, float64(execute_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userReplSlavePrivDesc, prometheus.GaugeValue, float64(repl_slave_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userReplClientPrivDesc, prometheus.GaugeValue, float64(repl_client_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userCreateViewPrivDesc, prometheus.GaugeValue, float64(create_view_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userShowViewPrivDesc, prometheus.GaugeValue, float64(show_view_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userCreateRoutinePrivDesc, prometheus.GaugeValue, float64(create_routine_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userAlterRoutinePrivDesc, prometheus.GaugeValue, float64(alter_routine_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userCreateUserPrivDesc, prometheus.GaugeValue, float64(create_user_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userEventPrivDesc, prometheus.GaugeValue, float64(event_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userTriggerPrivDesc, prometheus.GaugeValue, float64(trigger_priv), user, host)
			ch <- prometheus.MustNewConstMetric(userCreateTablespacePrivDesc, prometheus.GaugeValue, float64(create_tablespace_priv), user, host)
		}

		ch <- prometheus.MustNewConstMetric(userMaxQuestionsDesc, prometheus.GaugeValue, float64(max_questions), user, host)
		ch <- prometheus.MustNewConstMetric(userMaxUpdatesDesc, prometheus.GaugeValue, float64(max_updates), user, host)
		ch <- prometheus.MustNewConstMetric(userMaxConnectionsDesc, prometheus.GaugeValue, float64(max_connections), user, host)
		ch <- prometheus.MustNewConstMetric(userMaxUserConnectionsDesc, prometheus.GaugeValue, float64(max_user_connections), user, host)
	}

	return nil
}
