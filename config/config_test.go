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
	"fmt"
	"os"
	"testing"

	"github.com/go-kit/log"

	"github.com/smartystreets/goconvey/convey"
)

func TestValidateConfig(t *testing.T) {
	convey.Convey("Working config validation", t, func() {
		c := MySqlConfigHandler{
			Config: &Config{},
		}
		if err := c.ReloadConfig("testdata/client.cnf", "localhost:3306", "", true, log.NewNopLogger()); err != nil {
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
		c := MySqlConfigHandler{
			Config: &Config{},
		}
		if err := c.ReloadConfig("testdata/child_client.cnf", "localhost:3306", "", true, log.NewNopLogger()); err != nil {
			t.Error(err)
		}
		cfg := c.GetConfig()
		section, _ := cfg.Sections["client.server1"]
		convey.So(section.Password, convey.ShouldEqual, "abc")
	})

	convey.Convey("Environment variable / CLI flags", t, func() {
		c := MySqlConfigHandler{
			Config: &Config{},
		}
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		if err := c.ReloadConfig("", "testhost:5000", "testuser", true, log.NewNopLogger()); err != nil {
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
		c := MySqlConfigHandler{
			Config: &Config{},
		}
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		err := c.ReloadConfig("", "testhost", "testuser", true, log.NewNopLogger())
		convey.So(
			err,
			convey.ShouldBeError,
		)
	})

	convey.Convey("Config file precedence over environment variables", t, func() {
		c := MySqlConfigHandler{
			Config: &Config{},
		}
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		if err := c.ReloadConfig("testdata/client.cnf", "localhost:3306", "fakeuser", true, log.NewNopLogger()); err != nil {
			t.Error(err)
		}

		cfg := c.GetConfig()
		section := cfg.Sections["client"]
		convey.So(section.User, convey.ShouldEqual, "root")
		convey.So(section.Password, convey.ShouldEqual, "abc")
	})

	convey.Convey("Client without user", t, func() {
		c := MySqlConfigHandler{
			Config: &Config{},
		}
		os.Clearenv()
		err := c.ReloadConfig("testdata/missing_user.cnf", "localhost:3306", "", true, log.NewNopLogger())
		convey.So(
			err,
			convey.ShouldResemble,
			fmt.Errorf("no configuration found"),
		)
	})

	convey.Convey("Client without password", t, func() {
		c := MySqlConfigHandler{
			Config: &Config{},
		}
		os.Clearenv()
		if err := c.ReloadConfig("testdata/missing_password.cnf", "localhost:3306", "", true, log.NewNopLogger()); err != nil {
			t.Error(err)
		}

		cfg := c.GetConfig()
		section := cfg.Sections["client"]
		convey.So(section.User, convey.ShouldEqual, "abc")
		convey.So(section.Password, convey.ShouldEqual, "")
	})
}

func TestFormDSN(t *testing.T) {
	var (
		c = MySqlConfigHandler{
			Config: &Config{},
		}
		err error
		dsn string
	)

	convey.Convey("Host exporter dsn", t, func() {
		if err := c.ReloadConfig("testdata/client.cnf", "localhost:3306", "", false, log.NewNopLogger()); err != nil {
			t.Error(err)
		}
		convey.Convey("Default Client", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client"]
			if dsn, err = section.FormDSN(""); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "root:abc@tcp(server2:3306)/")
		})
		convey.Convey("Target specific with explicit port", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client.server1"]
			if dsn, err = section.FormDSN("server1:5000"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@tcp(server1:5000)/")
		})
		convey.Convey("UNIX domain socket", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client.server1"]
			if dsn, err = section.FormDSN("unix:///run/mysqld/mysqld.sock"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@unix(/run/mysqld/mysqld.sock)/")
		})
	})
}

func TestFormDSNWithSslSkipVerify(t *testing.T) {
	var (
		c = MySqlConfigHandler{
			Config: &Config{},
		}
		err error
		dsn string
	)

	convey.Convey("Host exporter dsn with tls skip verify", t, func() {
		if err := c.ReloadConfig("testdata/client.cnf", "localhost:3306", "", true, log.NewNopLogger()); err != nil {
			t.Error(err)
		}
		convey.Convey("Default Client", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client"]
			if dsn, err = section.FormDSN(""); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "root:abc@tcp(server2:3306)/?tls=skip-verify")
		})
		convey.Convey("Target specific with explicit port", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client.server1"]
			if dsn, err = section.FormDSN("server1:5000"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@tcp(server1:5000)/?tls=skip-verify")
		})
	})
}

func TestFormDSNWithCustomTls(t *testing.T) {
	var (
		c = MySqlConfigHandler{
			Config: &Config{},
		}
		err error
		dsn string
	)

	convey.Convey("Host exporter dsn with custom tls", t, func() {
		if err := c.ReloadConfig("testdata/client_custom_tls.cnf", "localhost:3306", "", false, log.NewNopLogger()); err != nil {
			t.Error(err)
		}
		convey.Convey("Target tls enabled", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client_tls_true"]
			if dsn, err = section.FormDSN(""); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "usr:pwd@tcp(server2:3306)/?tls=true")
		})

		convey.Convey("Target tls preferred", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client_tls_preferred"]
			if dsn, err = section.FormDSN(""); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "usr:pwd@tcp(server3:3306)/?tls=preferred")
		})

		convey.Convey("Target tls skip-verify", func() {
			cfg := c.GetConfig()
			section := cfg.Sections["client_tls_skip_verify"]
			if dsn, err = section.FormDSN(""); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "usr:pwd@tcp(server3:3306)/?tls=skip-verify")
		})

	})
}
