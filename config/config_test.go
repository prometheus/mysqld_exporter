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
		if err := c.ReloadConfig("testdata/client.cnf", "localhost", "", true, log.NewNopLogger()); err != nil {
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
		if err := c.ReloadConfig("testdata/child_client.cnf", "localhost", "", true, log.NewNopLogger()); err != nil {
			t.Error(err)
		}
		cfg := c.GetConfig()
		section, _ := cfg.Sections["client.server1"]
		convey.So(section.Password, convey.ShouldEqual, "abc")
	})

	convey.Convey("Environment variables / CLI arguments with port", t, func() {
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

	convey.Convey("Environment variables / CLI arguments without port", t, func() {
		c := MySqlConfigHandler{
			Config: &Config{},
		}
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		if err := c.ReloadConfig("", "testhost", "testuser", true, log.NewNopLogger()); err != nil {
			t.Error(err)
		}

		cfg := c.GetConfig()
		section := cfg.Sections["client"]
		convey.So(section.Host, convey.ShouldEqual, "testhost")
		convey.So(section.Port, convey.ShouldEqual, 3306)
		convey.So(section.User, convey.ShouldEqual, "testuser")
		convey.So(section.Password, convey.ShouldEqual, "supersecretpassword")
	})

	convey.Convey("Config file precedence over environment variables", t, func() {
		c := MySqlConfigHandler{
			Config: &Config{},
		}
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		if err := c.ReloadConfig("testdata/client.cnf", "localhost", "fakeuser", true, log.NewNopLogger()); err != nil {
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
		err := c.ReloadConfig("testdata/missing_user.cnf", "localhost", "", true, log.NewNopLogger())
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
		err := c.ReloadConfig("testdata/missing_password.cnf", "localhost", "", true, log.NewNopLogger())
		convey.So(
			err,
			convey.ShouldResemble,
			fmt.Errorf("no configuration found"),
		)
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
		if err := c.ReloadConfig("testdata/client.cnf", "localhost", "", true, log.NewNopLogger()); err != nil {
			t.Error(err)
		}
		convey.Convey("Default Client", func() {
			cfg := c.GetConfig()
			section, _ := cfg.Sections["client"]
			if dsn, err = section.FormDSN(""); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "root:abc@tcp(server2:3306)/")
		})
		convey.Convey("Target specific with explicit port", func() {
			cfg := c.GetConfig()
			section, _ := cfg.Sections["client.server1"]
			if dsn, err = section.FormDSN("server1:5000"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@tcp(server1:5000)/")
		})
		convey.Convey("Target specific without explicit port", func() {
			cfg := c.GetConfig()
			section, _ := cfg.Sections["client.server1"]
			if dsn, err = section.FormDSN("server1"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@tcp(server1:3306)/")
		})
	})
}
