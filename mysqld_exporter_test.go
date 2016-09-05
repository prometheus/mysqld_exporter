package main

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestParseMycnf(t *testing.T) {
	const (
		tcpConfig = `
			[client]
			user = root
			password = abc123
		`
		tcpConfig2 = `
			[client]
			user = root
			password = abc123
			port = 3308
		`
		socketConfig = `
			[client]
			user = user
			password = pass
			socket = /var/lib/mysql/mysql.sock
		`
		socketConfig2 = `
			[client]
			user = dude
			password = nopassword
			# host and port will not be used because of socket presence
			host = 1.2.3.4
			port = 3307
			socket = /var/lib/mysql/mysql.sock
		`
		remoteConfig = `
			[client]
			user = dude
			password = nopassword
			host = 1.2.3.4
			port = 3307
		`
		badConfig = `
			[client]
			user = root
		`
		badConfig2 = `
			[client]
			password = abc123
			socket = /var/lib/mysql/mysql.sock
		`
		badConfig3 = `
			[hello]
			world = ismine
		`
		badConfig4 = `
			[hello]
			world
		`
	)
	convey.Convey("Various .my.cnf configurations", t, func() {
		convey.Convey("Local tcp connection", func() {
			dsn, _ := parseMycnf([]byte(tcpConfig))
			convey.So(dsn, convey.ShouldEqual, "root:abc123@tcp(localhost:3306)/")
		})
		convey.Convey("Local tcp connection on non-default port", func() {
			dsn, _ := parseMycnf([]byte(tcpConfig2))
			convey.So(dsn, convey.ShouldEqual, "root:abc123@tcp(localhost:3308)/")
		})
		convey.Convey("Socket connection", func() {
			dsn, _ := parseMycnf([]byte(socketConfig))
			convey.So(dsn, convey.ShouldEqual, "user:pass@unix(/var/lib/mysql/mysql.sock)/")
		})
		convey.Convey("Socket connection ignoring defined host", func() {
			dsn, _ := parseMycnf([]byte(socketConfig2))
			convey.So(dsn, convey.ShouldEqual, "dude:nopassword@unix(/var/lib/mysql/mysql.sock)/")
		})
		convey.Convey("Remote connection", func() {
			dsn, _ := parseMycnf([]byte(remoteConfig))
			convey.So(dsn, convey.ShouldEqual, "dude:nopassword@tcp(1.2.3.4:3307)/")
		})
		convey.Convey("Missed user", func() {
			_, err := parseMycnf([]byte(badConfig))
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("Missed password", func() {
			_, err := parseMycnf([]byte(badConfig2))
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("No [client] section", func() {
			_, err := parseMycnf([]byte(badConfig3))
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("Invalid config", func() {
			_, err := parseMycnf([]byte(badConfig4))
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func TestGetMySQLVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	convey.Convey("MySQL version extract", t, func() {
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow(""))
		convey.So(getMySQLVersion(db), convey.ShouldEqual, 999)
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow("something"))
		convey.So(getMySQLVersion(db), convey.ShouldEqual, 999)
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow("10.1.17-MariaDB"))
		convey.So(getMySQLVersion(db), convey.ShouldEqual, 10.1)
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow("5.7.13-6-log"))
		convey.So(getMySQLVersion(db), convey.ShouldEqual, 5.7)
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow("5.6.30-76.3-56-log"))
		convey.So(getMySQLVersion(db), convey.ShouldEqual, 5.6)
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow("5.5.51-38.1"))
		convey.So(getMySQLVersion(db), convey.ShouldEqual, 5.5)
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}
