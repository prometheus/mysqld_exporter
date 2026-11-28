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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
	"github.com/smartystreets/goconvey/convey"
)

var _ interface{ Validate() error } = Config{}

func TestConfigDefaultsAndValidation(t *testing.T) {
	cfg := NewConfigWithDefaults()
	if cfg.ExporterLockWaitTimeoutSeconds != DefaultExporterLockWaitTimeoutSeconds {
		t.Fatalf("unexpected lock wait timeout: got %d, want %d", cfg.ExporterLockWaitTimeoutSeconds, DefaultExporterLockWaitTimeoutSeconds)
	}
	if cfg.ExporterMaxIdleConns != 1 {
		t.Errorf("max idle connections = %d, want 1", cfg.ExporterMaxIdleConns)
	}
	if cfg.ExporterConnMaxLifetime != 0 {
		t.Errorf("connection max lifetime = %v, want 0", cfg.ExporterConnMaxLifetime)
	}
	if cfg.Collectors["global_status"] != true {
		t.Fatal("global_status should be enabled by default")
	}
	if cfg.Collectors["info_schema.processlist"] != false {
		t.Fatal("info_schema.processlist should be disabled by default")
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing data source name to fail validation")
	}

	cfg.DataSourceName = "root@tcp(localhost:3306)/"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestConfigConnectionPoolValidation(t *testing.T) {
	for _, tt := range []struct {
		name        string
		maxIdle     int
		maxLifetime time.Duration
		wantErr     string
	}{
		{name: "disabled", maxIdle: 0, maxLifetime: 0},
		{name: "configured", maxIdle: 1, maxLifetime: 30 * time.Second},
		{name: "idle above open limit", maxIdle: 3, maxLifetime: time.Minute},
		{name: "negative idle limit", maxIdle: -1, wantErr: "exporter max idle connections must not be negative"},
		{name: "negative lifetime", maxIdle: 1, maxLifetime: -time.Second, wantErr: "exporter connection max lifetime must not be negative"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfigWithDefaults()
			cfg.DataSourceName = "root@tcp(localhost:3306)/"
			cfg.ExporterMaxIdleConns = tt.maxIdle
			cfg.ExporterConnMaxLifetime = tt.maxLifetime
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected validation error: %v", err)
				}
			} else if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("validation error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestNewAuthConfigHandler(t *testing.T) {
	registry := prometheus.NewRegistry()
	handler, err := NewAuthConfigHandler(registry)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if handler.GetConfig() == nil {
		t.Fatal("handler should initialize auth config")
	}

	if _, err := NewAuthConfigHandler(registry); err == nil {
		t.Fatal("expected duplicate metric registration to return an error")
	}
}

func TestValidateConfig(t *testing.T) {
	convey.Convey("Working config validation", t, func() {
		c := AuthConfigHandler{
			Config: &AuthConfig{},
		}
		if err := c.ReloadConfig("testdata/client.cnf", "localhost:3306", "", true, promslog.NewNopLogger()); err != nil {
			t.Error(err)
		}

		convey.Convey("Valid configuration", func() {
			cfg := c.GetConfig()
			convey.So(cfg.Sections, convey.ShouldContainKey, "client")
			convey.So(cfg.Sections, convey.ShouldContainKey, "client.server1")

			section, ok := cfg.Sections["client"]
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(section.User, convey.ShouldEqual, "root")
			convey.So(section.Password, convey.ShouldEqual, "abc")

			childSection, ok := cfg.Sections["client.server1"]
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(childSection.User, convey.ShouldEqual, "test")
			convey.So(childSection.Password, convey.ShouldEqual, "foo")

		})

		convey.Convey("False on non-existent section", func() {
			cfg := c.GetConfig()
			_, ok := cfg.Sections["fakeclient"]
			convey.So(ok, convey.ShouldBeFalse)
		})
	})

	convey.Convey("Inherit from parent section", t, func() {
		c := AuthConfigHandler{
			Config: &AuthConfig{},
		}
		if err := c.ReloadConfig("testdata/child_client.cnf", "localhost:3306", "", true, promslog.NewNopLogger()); err != nil {
			t.Error(err)
		}
		cfg := c.GetConfig()
		section := cfg.Sections["client.server1"]
		convey.So(section.Password, convey.ShouldEqual, "abc")
	})

	convey.Convey("Environment variable / CLI flags", t, func() {
		c := AuthConfigHandler{
			Config: &AuthConfig{},
		}
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		if err := c.ReloadConfig("", "testhost:5000", "testuser", true, promslog.NewNopLogger()); err != nil {
			t.Error(err)
		}

		cfg := c.GetConfig()
		section := cfg.Sections["client"]
		convey.So(section.Host, convey.ShouldEqual, "testhost")
		convey.So(section.Port, convey.ShouldEqual, 5000)
		convey.So(section.User, convey.ShouldEqual, "testuser")
		convey.So(section.Password, convey.ShouldEqual, "supersecretpassword")
	})

	convey.Convey("Environment variable / CLI flags error without port", t, func() {
		c := AuthConfigHandler{
			Config: &AuthConfig{},
		}
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		err := c.ReloadConfig("", "testhost", "testuser", true, promslog.NewNopLogger())
		convey.So(
			err,
			convey.ShouldBeError,
		)
	})

	convey.Convey("Unix socket address support", t, func() {
		c := AuthConfigHandler{
			Config: &AuthConfig{},
		}
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		if err := c.ReloadConfig("", "unix:///run/mysqld/mysqld.sock", "testuser", true, promslog.NewNopLogger()); err != nil {
			t.Error(err)
		}

		cfg := c.GetConfig()
		section := cfg.Sections["client"]
		convey.So(section.Socket, convey.ShouldEqual, "/run/mysqld/mysqld.sock")
		convey.So(section.User, convey.ShouldEqual, "testuser")
		convey.So(section.Password, convey.ShouldEqual, "supersecretpassword")
	})

	convey.Convey("Config file precedence over environment variables", t, func() {
		c := AuthConfigHandler{
			Config: &AuthConfig{},
		}
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		if err := c.ReloadConfig("testdata/client.cnf", "localhost:3306", "fakeuser", true, promslog.NewNopLogger()); err != nil {
			t.Error(err)
		}

		cfg := c.GetConfig()
		section := cfg.Sections["client"]
		convey.So(section.User, convey.ShouldEqual, "root")
		convey.So(section.Password, convey.ShouldEqual, "abc")
	})

	convey.Convey("Client without user", t, func() {
		c := AuthConfigHandler{
			Config: &AuthConfig{},
		}
		os.Clearenv()
		err := c.ReloadConfig("testdata/missing_user.cnf", "localhost:3306", "", true, promslog.NewNopLogger())
		convey.So(
			err,
			convey.ShouldResemble,
			fmt.Errorf("no configuration found"),
		)
	})

	convey.Convey("Client without password", t, func() {
		c := AuthConfigHandler{
			Config: &AuthConfig{},
		}
		os.Clearenv()
		if err := c.ReloadConfig("testdata/missing_password.cnf", "localhost:3306", "", true, promslog.NewNopLogger()); err != nil {
			t.Error(err)
		}

		cfg := c.GetConfig()
		section := cfg.Sections["client"]
		convey.So(section.User, convey.ShouldEqual, "abc")
		convey.So(section.Password, convey.ShouldEqual, "")
	})

	convey.Convey("Client with cleartext password enabled", t, func() {
		c := AuthConfigHandler{
			Config: &AuthConfig{},
		}
		os.Clearenv()
		if err := c.ReloadConfig("testdata/client.cnf", "localhost:3306", "", true, promslog.NewNopLogger()); err != nil {
			t.Error(err)
		}
		cfg := c.GetConfig()
		section := cfg.Sections["client.cleartextPlugin"]
		convey.So(section.User, convey.ShouldEqual, "test")
		convey.So(section.Password, convey.ShouldEqual, "foo")
		convey.So(section.EnableCleartextPlugin, convey.ShouldBeTrue)
	})

	convey.Convey("Client with TLS min version config higher than TLS max version config", t, func() {
		conf := MySqlConfig{
			User:          "test",
			TlsMinVersion: "TLSv1.3",
			TlsMaxVersion: "TLSv1.2",
		}
		os.Clearenv()
		err := conf.validateConfig()
		convey.So(
			err,
			convey.ShouldResemble,
			fmt.Errorf("tls-min-version must not be higher than tls-max-version: TLSv1.3 > TLSv1.2"),
		)
	})

	convey.Convey("Client with unknown TLS min version configuration", t, func() {
		conf := MySqlConfig{
			User:          "test",
			TlsMinVersion: "TLSv-something",
		}
		os.Clearenv()
		err := conf.validateConfig()
		convey.So(
			err,
			convey.ShouldResemble,
			fmt.Errorf("tls-min-version=TLSv-something is not allowed, use one of: TLSv1.0, TLSv1.1, TLSv1.2, TLSv1.3"),
		)
	})

	convey.Convey("Client with unknown TLS max version configuration", t, func() {
		conf := MySqlConfig{
			User:          "test",
			TlsMaxVersion: "TLSv-something",
		}
		os.Clearenv()
		err := conf.validateConfig()
		convey.So(
			err,
			convey.ShouldResemble,
			fmt.Errorf("tls-max-version=TLSv-something is not allowed, use one of: TLSv1.0, TLSv1.1, TLSv1.2, TLSv1.3"),
		)
	})

	convey.Convey("Expand variables", t, func() {
		c := MySqlConfigHandler{
			Config: &AuthConfig{},
		}
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		if err := c.ReloadConfig("testdata/expand_variables.cnf", "localhost:3306", "", true, promslog.NewNopLogger()); err != nil {
			t.Error(err)
		}

		cfg := c.GetConfig()
		section := cfg.Sections["client.server1"]
		convey.So(section.User, convey.ShouldEqual, "test")
		convey.So(section.Password, convey.ShouldEqual, "foo")

		section = cfg.Sections["client.env"]
		convey.So(section.User, convey.ShouldEqual, "test2")
		convey.So(section.Password, convey.ShouldEqual, "supersecretpassword")

		section = cfg.Sections["client.envBraces"]
		convey.So(section.User, convey.ShouldEqual, "test2")
		convey.So(section.Password, convey.ShouldEqual, "supersecretpassword")

		section = cfg.Sections["client.notExpandEnv"]
		convey.So(section.User, convey.ShouldEqual, "mysql_exporter")
		convey.So(section.Password, convey.ShouldEqual, "SECRET_PA$SWORD")

		section = cfg.Sections["client.envNotExists"]
		convey.So(section.User, convey.ShouldEqual, "test")
		convey.So(section.Password, convey.ShouldEqual, "")

		section = cfg.Sections["client.twoDollars"]
		convey.So(section.User, convey.ShouldEqual, "test")
		convey.So(section.Password, convey.ShouldEqual, "$MYSQLD_EXPORTER_PASSWORD")

		section = cfg.Sections["client.threeDollars"]
		convey.So(section.User, convey.ShouldEqual, "test")
		convey.So(section.Password, convey.ShouldEqual, "$supersecretpassword")

		section = cfg.Sections["client.startWithDollarButAnotherDollarSymbolAlsoExists"]
		convey.So(section.User, convey.ShouldEqual, "test2dollars")
		convey.So(section.Password, convey.ShouldEqual, "supersecretpassword")

		section = cfg.Sections["client.dollarAndBraces"]
		convey.So(section.User, convey.ShouldEqual, "testBraces")
		convey.So(section.Password, convey.ShouldEqual, "supersecretpasswordFoo")

		section = cfg.Sections["client.twoEnvVariables"]
		convey.So(section.User, convey.ShouldEqual, "test2Env")
		convey.So(section.Password, convey.ShouldEqual, "supersecretpasswordFoosupersecretpassword")
	})
}

func TestFormDSN(t *testing.T) {
	var (
		c = AuthConfigHandler{
			Config: &AuthConfig{},
		}
		err error
		dsn string
	)

	convey.Convey("Host exporter dsn", t, func() {
		if err := c.ReloadConfig("testdata/client.cnf", "localhost:3306", "", false, promslog.NewNopLogger()); err != nil {
			t.Error(err)
		}
		convey.Convey("Default Client", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client"]
			if dsn, err = section.FormDSN("", "client"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "root:abc@tcp(server2:3306)/")
		})
		convey.Convey("Target specific with explicit port", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client.server1"]
			if dsn, err = section.FormDSN("server1:5000", "client.server1"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@tcp(server1:5000)/")
		})
		convey.Convey("UNIX domain socket", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client.server1"]
			if dsn, err = section.FormDSN("unix:///run/mysqld/mysqld.sock", "client.server1"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@unix(/run/mysqld/mysqld.sock)/")
		})
		convey.Convey("With cleartext password enabled", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client.cleartextPlugin"]
			if dsn, err = section.FormDSN("", "client.cleartextPlugin"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@tcp(server2:3306)/?allowCleartextPasswords=true")
		})
	})
}

func TestFormDSNWithSslSkipVerify(t *testing.T) {
	var (
		c = AuthConfigHandler{
			Config: &AuthConfig{},
		}
		err error
		dsn string
	)

	convey.Convey("Host exporter dsn with tls skip verify", t, func() {
		if err := c.ReloadConfig("testdata/client.cnf", "localhost:3306", "", true, promslog.NewNopLogger()); err != nil {
			t.Error(err)
		}
		convey.Convey("Default Client", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client"]
			if dsn, err = section.FormDSN("", "client"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "root:abc@tcp(server2:3306)/?tls=skip-verify")
		})
		convey.Convey("Target specific with explicit port", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client.server1"]
			if dsn, err = section.FormDSN("server1:5000", "client.server1"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@tcp(server1:5000)/?tls=skip-verify")
		})
	})
}

func TestFormDSNWithCustomTls(t *testing.T) {
	var (
		c = AuthConfigHandler{
			Config: &AuthConfig{},
		}
		err error
		dsn string
	)

	convey.Convey("Host exporter dsn with custom tls", t, func() {
		if err := c.ReloadConfig("testdata/client_custom_tls.cnf", "localhost:3306", "", false, promslog.NewNopLogger()); err != nil {
			t.Error(err)
		}
		convey.Convey("Target tls enabled", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client_tls_true"]
			if dsn, err = section.FormDSN("", "client_tls_true"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "usr:pwd@tcp(server2:3306)/?tls=true")
		})

		convey.Convey("Target tls preferred", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client_tls_preferred"]
			if dsn, err = section.FormDSN("", "client_tls_preferred"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "usr:pwd@tcp(server3:3306)/?tls=preferred")
		})

		convey.Convey("Target tls skip-verify", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client_tls_skip_verify"]
			if dsn, err = section.FormDSN("", "client_tls_skip_verify"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "usr:pwd@tcp(server3:3306)/?tls=skip-verify")
		})

		convey.Convey("Target tls custom with TLS versions configured", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client_tls_with_version_config"]
			if dsn, err = section.FormDSN("", "client_tls_with_version_config"); err != nil {
				t.Error(err)
			}
			parsed, _ := mysql.ParseDSN(dsn)
			convey.So(parsed.TLS.MinVersion, convey.ShouldEqual, uint16(tls.VersionTLS12))
			convey.So(parsed.TLS.MaxVersion, convey.ShouldEqual, uint16(tls.VersionTLS13))
			convey.So(dsn, convey.ShouldEqual, "usr:pwd@tcp(server3:3306)/?tls=client_tls_with_version_config")
		})

		convey.Convey("Target tls custom with TLS server name configured", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client_tls_with_server_name"]
			if dsn, err = section.FormDSN("", "client_tls_with_server_name"); err != nil {
				t.Error(err)
			}
			parsed, _ := mysql.ParseDSN(dsn)
			convey.So(parsed.TLS.ServerName, convey.ShouldEqual, "mysql.example")
			convey.So(dsn, convey.ShouldEqual, "usr:pwd@tcp(server3:3306)/?tls=client_tls_with_server_name")
		})
	})
}

func TestFormDSNWithClientCertificate(t *testing.T) {
	// Test-only certificate and private key, generated with:
	// openssl ecparam -name prime256v1 -param_enc named_curve -genkey -noout -out client-key.pem
	// openssl req -new -x509 -key client-key.pem -out client-cert.pem -days 3650 -subj /CN=mysqld-exporter-test
	const certPEM = `-----BEGIN CERTIFICATE-----
MIIBLTCB1AIJAMDFherkB/X3MAoGCCqGSM49BAMCMB8xHTAbBgNVBAMMFG15c3Fs
ZC1leHBvcnRlci10ZXN0MB4XDTI2MTExNDE2MjEzMloXDTM2MTExMTE2MjEzMlow
HzEdMBsGA1UEAwwUbXlzcWxkLWV4cG9ydGVyLXRlc3QwWTATBgcqhkjOPQIBBggq
hkjOPQMBBwNCAAT2fitrxeqXuWlnyUkTRCuElMO38pJ1VGAFACl0xqK4gb9weMx7
urvm4nkLPKncwgaP4ChLhohn6/IS6pRu2JW5MAoGCCqGSM49BAMCA0gAMEUCIQC4
ld3rxVE+bKM7+pEMrPr6jOuwJGIYVIGvugoM86JNCgIgO3Je0bTXZ88/UsNGZbPz
fc5XSHwsp4SUixH9cpTt+nc=
-----END CERTIFICATE-----
`
	const keyPEM = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEINT02pTK1H717/3QsVRB0wLXxHKjUa9UfeO7nt79rlxjoAoGCCqGSM49
AwEHoUQDQgAE9n4ra8Xql7lpZ8lJE0QrhJTDt/KSdVRgBQApdMaiuIG/cHjMe7q7
5uJ5Czyp3MIGj+AoS4aIZ+vyEuqUbtiVuQ==
-----END EC PRIVATE KEY-----
`

	certFile := filepath.Join(t.TempDir(), "client-cert.pem")
	if err := os.WriteFile(certFile, []byte(certPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "client-key.pem")
	if err := os.WriteFile(keyFile, []byte(keyPEM), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name       string
		withCA     bool
		skipVerify bool
		tlsMode    string
	}{
		{name: "system roots"},
		{name: "custom CA", withCA: true},
		{name: "skip verification", skipVerify: true},
		{name: "custom CA and skip verification", withCA: true, skipVerify: true},
		{name: "TLS skip-verify mode", tlsMode: "skip-verify"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sectionName := t.Name()

			cfg := MySqlConfig{
				User:                  "exporter",
				SslCert:               certFile,
				SslKey:                keyFile,
				TlsInsecureSkipVerify: tt.skipVerify,
				Tls:                   tt.tlsMode,
			}
			if tt.withCA {
				cfg.SslCa = certFile
			}

			dsn, err := cfg.FormDSN("mysql.example:3306", sectionName)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { mysql.DeregisterTLSConfig(sectionName) })

			parsed, err := mysql.ParseDSN(dsn)
			if err != nil {
				t.Fatal(err)
			}

			if parsed.TLS == nil || len(parsed.TLS.Certificates) != 1 {
				t.Fatal("client certificate must enable TLS and be sent with or without ssl-ca")
			}

			wantSkipVerify := tt.skipVerify || tt.tlsMode == "skip-verify"
			if parsed.TLS.InsecureSkipVerify != wantSkipVerify {
				t.Fatal("TLS must respect certificate verification settings")
			}
		})
	}
}

func TestFormDSNWithInvalidClientCertificate(t *testing.T) {
	for _, tt := range []struct {
		name string
		cert string
		key  string
	}{
		{name: "missing key", cert: "missing-cert.pem"},
		{name: "missing certificate", key: "missing-key.pem"},
		{name: "unreadable pair", cert: "missing-cert.pem", key: "missing-key.pem"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := MySqlConfig{SslCert: tt.cert, SslKey: tt.key}
			if _, err := cfg.FormDSN("mysql.example:3306", t.Name()); err == nil {
				t.Fatal("invalid client certificate configuration must return an error")
			}
		})
	}
}

func TestFormDSNWithReservedTLSConfigName(t *testing.T) {
	cfg := MySqlConfig{TlsServerName: "mysql.example"}
	if _, err := cfg.FormDSN("mysql.example:3306", "true"); err == nil {
		t.Fatal("reserved TLS configuration name must return its registration error")
	}
}

func TestReloadConfigReportsFailure(t *testing.T) {
	ch, err := NewAuthConfigHandler(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	reloadSuccess := func() float64 {
		m := &dto.Metric{}
		if err := ch.configReloadSuccess.Write(m); err != nil {
			t.Fatal(err)
		}
		return m.GetGauge().GetValue()
	}

	t.Setenv("MYSQLD_EXPORTER_PASSWORD", "secret")
	if err := ch.ReloadConfig("", "localhost:3306", "user", true, promslog.NewNopLogger()); err != nil {
		t.Fatalf("valid reload failed: %v", err)
	}
	if got := reloadSuccess(); got != 1 {
		t.Fatalf("after a successful reload: got %v, want 1", got)
	}

	if err := ch.ReloadConfig("", "invalid-address", "user", true, promslog.NewNopLogger()); err == nil {
		t.Fatal("expected an error for an address without a port")
	}
	if got := reloadSuccess(); got != 0 {
		t.Fatalf("after a failed reload: got %v, want 0", got)
	}
}

func TestReloadConfigPasswordFile(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte("filesecret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	reload := func() (string, error) {
		c := AuthConfigHandler{Config: &AuthConfig{}}
		if err := c.ReloadConfig("", "localhost:3306", "user", true, promslog.NewNopLogger()); err != nil {
			return "", err
		}
		return c.GetConfig().Sections["client"].Password, nil
	}

	t.Run("reads the password from the file", func(t *testing.T) {
		t.Setenv(passwordEnv, "")
		t.Setenv(passwordFileEnv, passwordFile)
		password, err := reload()
		if err != nil {
			t.Fatal(err)
		}
		if password != "filesecret" {
			t.Fatalf("got password %q, want %q", password, "filesecret")
		}
	})

	t.Run("rejects both the variable and the file", func(t *testing.T) {
		t.Setenv(passwordEnv, "envsecret")
		t.Setenv(passwordFileEnv, passwordFile)
		if _, err := reload(); err == nil {
			t.Fatal("expected an error when both are set")
		}
	})

	t.Run("fails when the file can't be read", func(t *testing.T) {
		t.Setenv(passwordEnv, "")
		t.Setenv(passwordFileEnv, filepath.Join(t.TempDir(), "missing"))
		if _, err := reload(); err == nil {
			t.Fatal("expected an error for a missing file")
		}
	})
}
