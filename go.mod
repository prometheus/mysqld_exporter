module github.com/percona/mysqld_exporter

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/go-kit/log v0.2.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/google/uuid v1.3.0
	github.com/montanaflynn/stats v0.6.6
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.32.1
	github.com/prometheus/exporter-toolkit v0.7.1
	github.com/smartystreets/goconvey v1.7.2
	github.com/stretchr/testify v1.4.0
	github.com/tklauser/go-sysconf v0.3.10
	golang.org/x/sys v0.0.0-20220128215802-99c3d69c2c27
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/ini.v1 v1.66.4
	gopkg.in/yaml.v2 v2.4.0
)

go 1.15
