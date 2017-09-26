package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
	"net/url"
	"net"
	"syscall"

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

type binData struct {
	bin string
}

func TestBin(t *testing.T) {
	var err error
	binName := "mysqld_exporter"

	binDir, err := ioutil.TempDir("/tmp", binName+"-test-bindir-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := os.RemoveAll(binDir)
		if err != nil {
			t.Fatal(err)
		}
	}()

	importpath := "github.com/percona/mysqld_exporter/vendor/github.com/prometheus/common"
	bin := binDir + "/" + binName
	xVariables := map[string]string{
		importpath + "/version.Version":  "gotest-version",
		importpath + "/version.Branch":   "gotest-branch",
		importpath + "/version.Revision": "gotest-revision",
	}
	var ldflags []string
	for x, value := range xVariables {
		ldflags = append(ldflags, fmt.Sprintf("-X %s=%s", x, value))
	}
	cmd := exec.Command(
		"go",
		"build",
		"-o",
		bin,
		"-ldflags",
		strings.Join(ldflags, " "),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		t.Fatalf("Failed to build: %s", err)
	}

	data := binData{
		bin: bin,
	}
	tests := []func(*testing.T, binData){
		testLandingPage,
		testVersion,
	}
	t.Run(binName, func(t *testing.T) {
		for _, f := range tests {
			f := f // capture range variable
			fName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
			t.Run(fName, func(t *testing.T) {
				t.Parallel()
				f(t, data)
			})
		}
	})
}

func testVersion(t *testing.T, data binData) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		data.bin,
		"--version",
	)

	b := &bytes.Buffer{}
	cmd.Stdout = b
	cmd.Stderr = b

	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	expectedRegexp := `mysqld_exporter, version gotest-version \(branch: gotest-branch, revision: gotest-revision\)
  build user:
  build date:
  go version:       go1.+
`

	expectedScanner := bufio.NewScanner(bytes.NewBufferString(expectedRegexp))
	defer func() {
		if err := expectedScanner.Err(); err != nil {
			t.Fatal(err)
		}
	}()

	gotScanner := bufio.NewScanner(b)
	defer func() {
		if err := gotScanner.Err(); err != nil {
			t.Fatal(err)
		}
	}()

	for gotScanner.Scan() {
		if !expectedScanner.Scan() {
			t.Fatalf("didn't expected more data but got '%s'", gotScanner.Text())
		}
		ok, err := regexp.MatchString(expectedScanner.Text(), gotScanner.Text())
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("'%s' does not match regexp '%s'", gotScanner.Text(), expectedScanner.Text())
		}
	}

	if expectedScanner.Scan() {
		t.Errorf("expected '%s' but didn't got more data", expectedScanner.Text())
	}
}

func testLandingPage(t *testing.T, data binData) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		data.bin,
	)
	cmd.Env = append(os.Environ(), "DATA_SOURCE_NAME=127.0.0.1:3306")

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()

	// Get the main page, but we need to wait a bit for http server
	var resp *http.Response
	var err error
	for i:=0; i<= 10; i++ {
		// Try to get main page
		resp, err = http.Get("http://127.0.0.1:9104")
		if err == nil {
			break
		}

		// If there is a syscall.ECONNREFUSED error (web server not available) then retry
		if urlError, ok := err.(*url.Error); ok {
			if opError, ok := urlError.Err.(*net.OpError); ok {
				if osSyscallError, ok := opError.Err.(*os.SyscallError); ok {
					if osSyscallError.Err == syscall.ECONNREFUSED {
						time.Sleep(1 * time.Second)
						continue
					}
				}
			}
		}

		t.Fatalf("%#v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)

	expected := `<html>
<head><title>MySQLd 3-in-1 exporter</title></head>
<body>
<h1>MySQL 3-in-1 exporter</h1>
<li><a href="/metrics-hr">high-res metrics</a></li>
<li><a href="/metrics-mr">medium-res metrics</a></li>
<li><a href="/metrics-lr">low-res metrics</a></li>
</body>
</html>
`
	if got != expected {
		t.Fatalf("got '%s' but expected '%s'", got, expected)
	}
}
