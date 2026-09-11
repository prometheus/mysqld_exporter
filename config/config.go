// Copyright 2022 The Prometheus Authors
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

package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"

	"gopkg.in/ini.v1"
)

var (
	opts = ini.LoadOptions{
		// Do not error on nonexistent file to allow empty string as filename input
		Loose: true,
		// MySQL ini file can have boolean keys.
		AllowBooleanKeys: true,
		// Ignore the # character in the line to avoid password parsing failure when the MySQL password contains the # symbol
		IgnoreInlineComment: true,
		// Remove the first and last quotation marks
		UnescapeValueDoubleQuotes: true,
	}

	err error

	// Use the same pattern of TLS version strings as the mysql client binary.
	tlsVersions = map[string]uint16{
		"TLSv1.0": tls.VersionTLS10,
		"TLSv1.1": tls.VersionTLS11,
		"TLSv1.2": tls.VersionTLS12,
		"TLSv1.3": tls.VersionTLS13,
	}
)

const (
	DefaultTimeoutOffsetSeconds           = 0.25
	DefaultExporterLockWaitTimeoutSeconds = 2
	DefaultEnableExporterLockTimeout      = true
	DefaultSlowLogFilter                  = false
	DefaultExporterQueryTimeout           = 0
	DefaultExporterMaxOpenConns           = 2

	DefaultHeartbeatDatabase = "heartbeat"
	DefaultHeartbeatTable    = "heartbeat"
	DefaultHeartbeatUTC      = false

	DefaultInfoSchemaProcesslistMinTime         = 0
	DefaultInfoSchemaProcesslistProcessesByUser = true
	DefaultInfoSchemaProcesslistProcessesByHost = true

	DefaultInfoSchemaTablesDatabases = "*"

	DefaultPerfSchemaEventsStatementsLimit           = 250
	DefaultPerfSchemaEventsStatementsTimeLimit       = 86400
	DefaultPerfSchemaEventsStatementsDigestTextLimit = 120

	DefaultPerfSchemaFileInstancesFilter       = ".*"
	DefaultPerfSchemaFileInstancesRemovePrefix = "/var/lib/mysql/"
	DefaultPerfSchemaMemoryEventsRemovePrefix  = "memory/"

	DefaultMysqlUserPrivileges = false
)

type Config struct {
	DataSourceName                 string
	Collectors                     map[string]bool
	TimeoutOffsetSeconds           float64
	EnableExporterLockWaitTimeout  bool
	ExporterLockWaitTimeoutSeconds int
	SlowLogFilter                  bool
	ExporterQueryTimeout           time.Duration
	ExporterMaxOpenConns           int
	Heartbeat                      HeartbeatConfig
	InfoSchemaProcesslist          InfoSchemaProcesslistConfig
	InfoSchemaTables               InfoSchemaTablesConfig
	PerfSchemaEventsStatements     PerfSchemaEventsStatementsConfig
	PerfSchemaFileInstances        PerfSchemaFileInstancesConfig
	PerfSchemaMemoryEvents         PerfSchemaMemoryEventsConfig
	MysqlUser                      MysqlUserConfig
}

type EmptyConfig struct{}

type HeartbeatConfig struct {
	Database string
	Table    string
	UTC      bool
}

type InfoSchemaProcesslistConfig struct {
	MinTime         int
	ProcessesByUser bool
	ProcessesByHost bool
}

type InfoSchemaTablesConfig struct {
	Databases string
}

type PerfSchemaEventsStatementsConfig struct {
	Limit           int
	TimeLimit       int
	DigestTextLimit int
	ExcludeSchemas  []string
}

type PerfSchemaFileInstancesConfig struct {
	Filter       string
	RemovePrefix string
}

type PerfSchemaMemoryEventsConfig struct {
	RemovePrefix string
}

type MysqlUserConfig struct {
	Privileges bool
}

func NewConfigWithDefaults() Config {
	return Config{
		Collectors:                     DefaultCollectorConfig(),
		TimeoutOffsetSeconds:           DefaultTimeoutOffsetSeconds,
		EnableExporterLockWaitTimeout:  DefaultEnableExporterLockTimeout,
		ExporterLockWaitTimeoutSeconds: DefaultExporterLockWaitTimeoutSeconds,
		SlowLogFilter:                  DefaultSlowLogFilter,
		ExporterQueryTimeout:           DefaultExporterQueryTimeout,
		ExporterMaxOpenConns:           DefaultExporterMaxOpenConns,
		Heartbeat: HeartbeatConfig{
			Database: DefaultHeartbeatDatabase,
			Table:    DefaultHeartbeatTable,
			UTC:      DefaultHeartbeatUTC,
		},
		InfoSchemaProcesslist: InfoSchemaProcesslistConfig{
			MinTime:         DefaultInfoSchemaProcesslistMinTime,
			ProcessesByUser: DefaultInfoSchemaProcesslistProcessesByUser,
			ProcessesByHost: DefaultInfoSchemaProcesslistProcessesByHost,
		},
		InfoSchemaTables: InfoSchemaTablesConfig{
			Databases: DefaultInfoSchemaTablesDatabases,
		},
		PerfSchemaEventsStatements: PerfSchemaEventsStatementsConfig{
			Limit:           DefaultPerfSchemaEventsStatementsLimit,
			TimeLimit:       DefaultPerfSchemaEventsStatementsTimeLimit,
			DigestTextLimit: DefaultPerfSchemaEventsStatementsDigestTextLimit,
		},
		PerfSchemaFileInstances: PerfSchemaFileInstancesConfig{
			Filter:       DefaultPerfSchemaFileInstancesFilter,
			RemovePrefix: DefaultPerfSchemaFileInstancesRemovePrefix,
		},
		PerfSchemaMemoryEvents: PerfSchemaMemoryEventsConfig{
			RemovePrefix: DefaultPerfSchemaMemoryEventsRemovePrefix,
		},
		MysqlUser: MysqlUserConfig{
			Privileges: DefaultMysqlUserPrivileges,
		},
	}
}

func DefaultCollectorConfig() map[string]bool {
	return map[string]bool{
		"global_status":                                    true,
		"global_variables":                                 true,
		"slave_status":                                     true,
		"info_schema.processlist":                          false,
		"mysql.user":                                       false,
		"info_schema.tables":                               false,
		"info_schema.innodb_tablespaces":                   false,
		"info_schema.innodb_metrics":                       false,
		"auto_increment.columns":                           false,
		"binlog_size":                                      false,
		"perf_schema.tableiowaits":                         false,
		"perf_schema.indexiowaits":                         false,
		"perf_schema.tablelocks":                           false,
		"perf_schema.eventsstatements":                     false,
		"perf_schema.eventsstatementssum":                  false,
		"perf_schema.eventswaits":                          false,
		"perf_schema.file_events":                          false,
		"perf_schema.file_instances":                       false,
		"perf_schema.memory_events":                        false,
		"perf_schema.replication_group_members":            false,
		"perf_schema.replication_group_member_stats":       false,
		"perf_schema.replication_applier_status_by_worker": false,
		"sys.user_summary":                                 false,
		"info_schema.userstats":                            false,
		"info_schema.clientstats":                          false,
		"info_schema.tablestats":                           false,
		"info_schema.schemastats":                          false,
		"info_schema.innodb_cmp":                           true,
		"info_schema.innodb_cmpmem":                        true,
		"info_schema.query_response_time":                  true,
		"engine_tokudb_status":                             false,
		"engine_innodb_status":                             false,
		"heartbeat":                                        false,
		"slave_hosts":                                      false,
		"info_schema.replica_host":                         false,
		"info_schema.rocksdb_perf_context":                 false,
	}
}

func (c Config) Validate() error {
	if c.DataSourceName == "" {
		return fmt.Errorf("data source name must not be empty")
	}
	if c.TimeoutOffsetSeconds < 0 {
		return fmt.Errorf("timeout offset must not be negative")
	}
	if c.ExporterLockWaitTimeoutSeconds < 0 {
		return fmt.Errorf("exporter lock wait timeout must not be negative")
	}
	if c.ExporterQueryTimeout < 0 {
		return fmt.Errorf("exporter query timeout must not be negative")
	}
	if c.ExporterMaxOpenConns < 1 {
		return fmt.Errorf("exporter max open connections must be at least 1")
	}
	if c.Heartbeat.Database == "" {
		return fmt.Errorf("heartbeat database must not be empty")
	}
	if c.Heartbeat.Table == "" {
		return fmt.Errorf("heartbeat table must not be empty")
	}
	if c.InfoSchemaProcesslist.MinTime < 0 {
		return fmt.Errorf("info_schema processlist min time must not be negative")
	}
	if c.InfoSchemaTables.Databases == "" {
		return fmt.Errorf("info_schema tables databases must not be empty")
	}
	if c.PerfSchemaEventsStatements.Limit <= 0 {
		return fmt.Errorf("perf_schema events statements limit must be greater than zero")
	}
	if c.PerfSchemaEventsStatements.TimeLimit <= 0 {
		return fmt.Errorf("perf_schema events statements time limit must be greater than zero")
	}
	if c.PerfSchemaEventsStatements.DigestTextLimit <= 0 {
		return fmt.Errorf("perf_schema events statements digest text limit must be greater than zero")
	}
	if c.PerfSchemaFileInstances.Filter == "" {
		return fmt.Errorf("perf_schema file instances filter must not be empty")
	}
	for name := range c.Collectors {
		if _, ok := DefaultCollectorConfig()[name]; !ok {
			return fmt.Errorf("unknown collector %q", name)
		}
	}

	return nil
}

type AuthConfig struct {
	Sections map[string]MySqlConfig
}

type MySqlConfig struct {
	User                  string `ini:"user"`
	Password              string `ini:"password"`
	Host                  string `ini:"host"`
	Port                  int    `ini:"port"`
	Socket                string `ini:"socket"`
	EnableCleartextPlugin bool   `ini:"enable-cleartext-plugin"`
	SslCa                 string `ini:"ssl-ca"`
	SslCert               string `ini:"ssl-cert"`
	SslKey                string `ini:"ssl-key"`
	TlsInsecureSkipVerify bool   `ini:"ssl-skip-verfication"` //nolint:misspell
	Tls                   string `ini:"tls"`
	TlsMinVersion         string `ini:"tls-min-version"`
	TlsMaxVersion         string `ini:"tls-max-version"`
	TlsServerName         string `ini:"tls-server-name"`
}

type AuthConfigHandler struct {
	sync.RWMutex
	TlsInsecureSkipVerify bool
	Config                *AuthConfig
	configReloadSuccess   prometheus.Gauge
	configReloadSeconds   prometheus.Gauge
}

type MySqlConfigHandler = AuthConfigHandler

func NewAuthConfigHandler(registerer prometheus.Registerer) (*AuthConfigHandler, error) {
	if registerer == nil {
		return nil, errors.New("registerer is required")
	}
	ch := &AuthConfigHandler{
		Config: &AuthConfig{},
		configReloadSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "mysqld_exporter",
			Name:      "config_last_reload_successful",
			Help:      "Mysqld exporter config loaded successfully.",
		}),
		configReloadSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "mysqld_exporter",
			Name:      "config_last_reload_success_timestamp_seconds",
			Help:      "Timestamp of the last successful configuration reload.",
		}),
	}
	collectors := []prometheus.Collector{ch.configReloadSuccess, ch.configReloadSeconds}
	for i, collector := range collectors {
		if err := registerer.Register(collector); err != nil {
			for _, registered := range collectors[:i] {
				registerer.Unregister(registered)
			}
			return nil, fmt.Errorf("register auth config metric: %w", err)
		}
	}
	return ch, nil
}

func (ch *AuthConfigHandler) GetConfig() *AuthConfig {
	ch.RLock()
	defer ch.RUnlock()
	return ch.Config
}

func (ch *AuthConfigHandler) ReloadConfig(filename string, mysqldAddress string, mysqldUser string, tlsInsecureSkipVerify bool, logger *slog.Logger) error {
	var host, port string
	defer func() {
		ch.observeReload(err)
	}()

	cfg, err := ini.LoadSources(
		opts,
		[]byte("[client]\npassword = ${MYSQLD_EXPORTER_PASSWORD}\n"),
		filename,
	)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", filename, err)
	}

	if clientSection := cfg.Section("client"); clientSection != nil {
		// Check if mysqldAddress is a unix socket
		if prefix := "unix://"; strings.HasPrefix(mysqldAddress, prefix) {
			socketPath := mysqldAddress[len(prefix):]
			if cfgSocket := clientSection.Key("socket"); cfgSocket.String() == "" {
				cfgSocket.SetValue(socketPath)
			}
		} else {
			// Parse as TCP address (host:port)
			if host, port, err = net.SplitHostPort(mysqldAddress); err != nil {
				return fmt.Errorf("failed to parse address: %w", err)
			}
			if cfgHost := clientSection.Key("host"); cfgHost.String() == "" {
				cfgHost.SetValue(host)
			}
			if cfgPort := clientSection.Key("port"); cfgPort.String() == "" {
				cfgPort.SetValue(port)
			}
		}
		if cfgUser := clientSection.Key("user"); cfgUser.String() == "" {
			cfgUser.SetValue(mysqldUser)
		}
	}

	cfg.ValueMapper = func(val string) string {
		return os.Expand(val, func(name string) string {
			if name == "$" {
				return "$"
			}
			return os.Getenv(name)
		})
	}
	config := &AuthConfig{}
	m := make(map[string]MySqlConfig)
	for _, sec := range cfg.Sections() {
		sectionName := sec.Name()

		if sectionName == "DEFAULT" {
			continue
		}

		mysqlcfg := &MySqlConfig{
			TlsInsecureSkipVerify: tlsInsecureSkipVerify,
		}

		err = sec.StrictMapTo(mysqlcfg)
		if err != nil {
			logger.Error("failed to parse config", "section", sectionName, "err", err)
			continue
		}
		if err := mysqlcfg.validateConfig(); err != nil {
			logger.Error("failed to validate config", "section", sectionName, "err", err)
			continue
		}

		m[sectionName] = *mysqlcfg
	}
	config.Sections = m
	if len(config.Sections) == 0 {
		return fmt.Errorf("no configuration found")
	}
	ch.Lock()
	ch.Config = config
	ch.Unlock()
	return nil
}

func (ch *AuthConfigHandler) observeReload(err error) {
	if ch.configReloadSuccess == nil {
		return
	}
	if err != nil {
		ch.configReloadSuccess.Set(0)
		return
	}
	ch.configReloadSuccess.Set(1)
	if ch.configReloadSeconds != nil {
		ch.configReloadSeconds.SetToCurrentTime()
	}
}

func (m MySqlConfig) validateConfig() error {
	if m.User == "" {
		return fmt.Errorf("no user specified in section or parent")
	}

	allowedTLSVersions := strings.Join(slices.Sorted(maps.Keys(tlsVersions)), ", ")
	if _, ok := tlsVersions[m.TlsMinVersion]; !ok && m.TlsMinVersion != "" {
		return fmt.Errorf("tls-min-version=%s is not allowed, use one of: %s", m.TlsMinVersion, allowedTLSVersions)
	}
	if _, ok := tlsVersions[m.TlsMaxVersion]; !ok && m.TlsMaxVersion != "" {
		return fmt.Errorf("tls-max-version=%s is not allowed, use one of: %s", m.TlsMaxVersion, allowedTLSVersions)
	}
	if m.TlsMinVersion != "" && m.TlsMaxVersion != "" {
		if tlsVersions[m.TlsMinVersion] > tlsVersions[m.TlsMaxVersion] {
			return fmt.Errorf("tls-min-version must not be higher than tls-max-version: %s > %s", m.TlsMinVersion, m.TlsMaxVersion)
		}
	}

	return nil
}

func (m MySqlConfig) FormDSN(target string, configSectionName string) (string, error) {
	config := mysql.NewConfig()
	config.User = m.User
	config.Passwd = m.Password
	config.Net = "tcp"
	if target == "" {
		if m.Socket == "" {
			host := "127.0.0.1"
			if m.Host != "" {
				host = m.Host
			}
			port := "3306"
			if m.Port != 0 {
				port = strconv.Itoa(m.Port)
			}
			config.Addr = net.JoinHostPort(host, port)
		} else {
			config.Net = "unix"
			config.Addr = m.Socket
		}
	} else if prefix := "unix://"; strings.HasPrefix(target, prefix) {
		config.Net = "unix"
		config.Addr = target[len(prefix):]
	} else {
		if _, _, err = net.SplitHostPort(target); err != nil {
			return "", fmt.Errorf("failed to parse target: %s", err)
		}
		config.Addr = target
	}

	if m.TlsInsecureSkipVerify {
		config.TLSConfig = "skip-verify"
	} else {
		config.TLSConfig = m.Tls

		hasCustomTLS := m.SslCa != "" ||
			m.TlsMinVersion != "" ||
			m.TlsMaxVersion != "" ||
			m.TlsServerName != ""

		if hasCustomTLS {
			if err := m.CustomizeTLS(configSectionName); err != nil {
				err = fmt.Errorf("failed to register a custom TLS configuration for mysql dsn: %w", err)
				return "", err
			}
			config.TLSConfig = configSectionName
		}
	}

	if m.EnableCleartextPlugin {
		config.AllowCleartextPasswords = true
	}

	return config.FormatDSN(), nil
}

func (m MySqlConfig) CustomizeTLS(configSectionName string) error {
	var tlsCfg tls.Config
	if m.SslCa != "" {
		caBundle := x509.NewCertPool()
		pemCA, err := os.ReadFile(m.SslCa)
		if err != nil {
			return err
		}
		if ok := caBundle.AppendCertsFromPEM(pemCA); ok {
			tlsCfg.RootCAs = caBundle
		} else {
			return fmt.Errorf("failed parse pem-encoded CA certificates from %s", m.SslCa)
		}
		if m.SslCert != "" && m.SslKey != "" {
			certPairs := make([]tls.Certificate, 0, 1)
			keypair, err := tls.LoadX509KeyPair(m.SslCert, m.SslKey)
			if err != nil {
				return fmt.Errorf("failed to parse pem-encoded SSL cert %s or SSL key %s: %w",
					m.SslCert, m.SslKey, err)
			}
			certPairs = append(certPairs, keypair)
			tlsCfg.Certificates = certPairs
		}
	}
	if m.TlsMinVersion != "" {
		tlsCfg.MinVersion = tlsVersions[m.TlsMinVersion]
	}
	if m.TlsMaxVersion != "" {
		tlsCfg.MaxVersion = tlsVersions[m.TlsMaxVersion]
	}
	if m.TlsServerName != "" {
		tlsCfg.ServerName = m.TlsServerName
	}
	tlsCfg.InsecureSkipVerify = m.TlsInsecureSkipVerify
	mysql.RegisterTLSConfig(configSectionName, &tlsCfg)
	return nil
}
