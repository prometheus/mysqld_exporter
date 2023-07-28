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

func TestMycnfValidateConfig(t *testing.T) {
	convey.Convey("Working config validation", t, func() {
		c := NewMycnfReloader(&MycnfReloaderOpts{
			DefaultMysqldAddress:         "localhost:3306",
			DefaultMysqldUser:            "",
			DefaultTlsInsecureSkipVerify: true,
			Logger:                       log.NewNopLogger(),
			MycnfPath:                    "testdata/client.cnf",
		})
		if err := c.Reload(); err != nil {
			t.Error(err)
		}

		convey.Convey("Valid configuration", func() {
			mycnf := c.Mycnf()
			convey.So(mycnf, convey.ShouldContainKey, "client")
			convey.So(mycnf, convey.ShouldContainKey, "client.server1")

			section, ok := mycnf["client"]
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(section.User, convey.ShouldEqual, "root")
			convey.So(section.Password, convey.ShouldEqual, "abc")

			childSection, ok := mycnf["client.server1"]
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(childSection.User, convey.ShouldEqual, "test")
			convey.So(childSection.Password, convey.ShouldEqual, "foo")

		})

		convey.Convey("False on non-existent section", func() {
			mycnf := c.Mycnf()
			_, ok := mycnf["fakeclient"]
			convey.So(ok, convey.ShouldBeFalse)
		})
	})

	convey.Convey("Inherit from parent section", t, func() {
		c := NewMycnfReloader(&MycnfReloaderOpts{
			DefaultMysqldAddress:         "localhost:3306",
			DefaultMysqldUser:            "",
			DefaultTlsInsecureSkipVerify: true,
			MycnfPath:                    "testdata/child_client.cnf",
			Logger:                       log.NewNopLogger(),
		})
		if err := c.Reload(); err != nil {
			t.Error(err)
		}
		mycnf := c.Mycnf()
		section, _ := mycnf["client.server1"]
		convey.So(section.Password, convey.ShouldEqual, "abc")
	})

	convey.Convey("Environment variable / CLI flags", t, func() {
		c := NewMycnfReloader(&MycnfReloaderOpts{
			DefaultMysqldAddress:         "testhost:5000",
			DefaultMysqldUser:            "testuser",
			DefaultTlsInsecureSkipVerify: true,
			Logger:                       log.NewNopLogger(),
			MycnfPath:                    "",
		})
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		if err := c.Reload(); err != nil {
			t.Error(err)
		}

		mycnf := c.Mycnf()
		section := mycnf["client"]
		convey.So(section.Host, convey.ShouldEqual, "testhost")
		convey.So(section.Port, convey.ShouldEqual, 5000)
		convey.So(section.User, convey.ShouldEqual, "testuser")
		convey.So(section.Password, convey.ShouldEqual, "supersecretpassword")
	})

	convey.Convey("Environment variable / CLI flags error without port", t, func() {
		c := NewMycnfReloader(&MycnfReloaderOpts{
			DefaultMysqldAddress:         "testhost",
			DefaultMysqldUser:            "testuser",
			DefaultTlsInsecureSkipVerify: true,
			Logger:                       log.NewNopLogger(),
			MycnfPath:                    "",
		})
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		err := c.Reload()
		convey.So(
			err,
			convey.ShouldBeError,
		)
	})

	convey.Convey("Config file precedence over environment variables", t, func() {
		c := NewMycnfReloader(&MycnfReloaderOpts{
			DefaultMysqldAddress:         "localhost:3306",
			DefaultMysqldUser:            "fakeuser",
			DefaultTlsInsecureSkipVerify: true,
			Logger:                       log.NewNopLogger(),
			MycnfPath:                    "testdata/client.cnf",
		})
		os.Setenv("MYSQLD_EXPORTER_PASSWORD", "supersecretpassword")
		if err := c.Reload(); err != nil {
			t.Error(err)
		}

		mycnf := c.Mycnf()
		section := mycnf["client"]
		convey.So(section.User, convey.ShouldEqual, "root")
		convey.So(section.Password, convey.ShouldEqual, "abc")
	})

	convey.Convey("Client without user", t, func() {
		c := NewMycnfReloader(&MycnfReloaderOpts{
			DefaultMysqldAddress:         "localhost:3306",
			DefaultMysqldUser:            "",
			DefaultTlsInsecureSkipVerify: true,
			Logger:                       log.NewNopLogger(),
			MycnfPath:                    "testdata/missing_user.cnf",
		})
		os.Clearenv()
		err := c.Reload()
		convey.So(
			err,
			convey.ShouldResemble,
			fmt.Errorf("no configuration found"),
		)
	})

	convey.Convey("Client without password", t, func() {
		c := NewMycnfReloader(&MycnfReloaderOpts{
			MycnfPath:                    "testdata/missing_password.cnf",
			DefaultMysqldAddress:         "localhost:3306",
			DefaultMysqldUser:            "",
			DefaultTlsInsecureSkipVerify: true,
			Logger:                       log.NewNopLogger(),
		})
		os.Clearenv()
		if err := c.Reload(); err != nil {
			t.Error(err)
		}

		mycnf := c.Mycnf()
		section := mycnf["client"]
		convey.So(section.User, convey.ShouldEqual, "abc")
		convey.So(section.Password, convey.ShouldEqual, "")
	})
}

func TestMycnfSectionFormDSN(t *testing.T) {
	var (
		c = NewMycnfReloader(&MycnfReloaderOpts{
			DefaultMysqldAddress:         "localhost:3306",
			DefaultMysqldUser:            "",
			DefaultTlsInsecureSkipVerify: false,
			Logger:                       log.NewNopLogger(),
			MycnfPath:                    "testdata/client.cnf",
		})
		err error
		dsn string
	)

	convey.Convey("Host exporter dsn", t, func() {
		if err := c.Reload(); err != nil {
			t.Error(err)
		}
		convey.Convey("Default Client", func() {
			mycnf := c.Mycnf()
			section := mycnf["client"]
			if dsn, err = section.FormDSN(""); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "root:abc@tcp(server2:3306)/")
		})
		convey.Convey("Target specific with explicit port", func() {
			mycnf := c.Mycnf()
			section := mycnf["client.server1"]
			if dsn, err = section.FormDSN("server1:5000"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@tcp(server1:5000)/")
		})
		convey.Convey("UNIX domain socket", func() {
			mycnf := c.Mycnf()
			section := mycnf["client.server1"]
			if dsn, err = section.FormDSN("unix:///run/mysqld/mysqld.sock"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@unix(/run/mysqld/mysqld.sock)/")
		})
	})
}

func TestMycnfSectionFormDSNWithSslSkipVerify(t *testing.T) {
	var (
		c = NewMycnfReloader(&MycnfReloaderOpts{
			DefaultMysqldAddress:         "localhost:3306",
			DefaultMysqldUser:            "",
			DefaultTlsInsecureSkipVerify: true,
			MycnfPath:                    "testdata/client.cnf",
			Logger:                       log.NewNopLogger(),
		})
		err error
		dsn string
	)

	convey.Convey("Host exporter dsn with tls skip verify", t, func() {
		if err := c.Reload(); err != nil {
			t.Error(err)
		}
		convey.Convey("Default Client", func() {
			mycnf := c.Mycnf()
			section := mycnf["client"]
			if dsn, err = section.FormDSN(""); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "root:abc@tcp(server2:3306)/?tls=skip-verify")
		})
		convey.Convey("Target specific with explicit port", func() {
			mycnf := c.Mycnf()
			section := mycnf["client.server1"]
			if dsn, err = section.FormDSN("server1:5000"); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "test:foo@tcp(server1:5000)/?tls=skip-verify")
		})
	})
}

func TestMycnfSectionFormDSNWithCustomTls(t *testing.T) {
	var (
		c = NewMycnfReloader(&MycnfReloaderOpts{
			DefaultMysqldAddress:         "localhost:3306",
			DefaultMysqldUser:            "",
			DefaultTlsInsecureSkipVerify: false,
			Logger:                       log.NewNopLogger(),
			MycnfPath:                    "testdata/client_custom_tls.cnf",
		})
		err error
		dsn string
	)

	convey.Convey("Host exporter dsn with custom tls", t, func() {
		if err := c.Reload(); err != nil {
			t.Error(err)
		}
		convey.Convey("Target tls enabled", func() {
			mycnf := c.Mycnf()
			section := mycnf["client_tls_true"]
			if dsn, err = section.FormDSN(""); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "usr:pwd@tcp(server2:3306)/?tls=true")
		})

		convey.Convey("Target tls preferred", func() {
			mycnf := c.Mycnf()
			section := mycnf["client_tls_preferred"]
			if dsn, err = section.FormDSN(""); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "usr:pwd@tcp(server3:3306)/?tls=preferred")
		})

		convey.Convey("Target tls skip-verify", func() {
			mycnf := c.Mycnf()
			section := mycnf["client_tls_skip_verify"]
			if dsn, err = section.FormDSN(""); err != nil {
				t.Error(err)
			}
			convey.So(dsn, convey.ShouldEqual, "usr:pwd@tcp(server3:3306)/?tls=skip-verify")
		})

	})
}
