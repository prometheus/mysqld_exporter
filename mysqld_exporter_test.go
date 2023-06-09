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

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/mysqld_exporter/collector"
)

// bin stores information about path of executable and attached port
type bin struct {
	path string
	port int
}

// TestBin builds, runs and tests binary.
func TestBin(t *testing.T) {
	var err error
	binName := "mysqld_exporter"

	binDir, err := os.MkdirTemp("/tmp", binName+"-test-bindir-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := os.RemoveAll(binDir)
		if err != nil {
			t.Fatal(err)
		}
	}()

	importpath := "github.com/prometheus/common"
	path := binDir + "/" + binName
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
		path,
		"-ldflags",
		strings.Join(ldflags, " "),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		t.Fatalf("Failed to build: %s", err)
	}

	tests := []func(*testing.T, bin){
		testLanding,
		testProbe,
	}

	portStart := 56000
	t.Run(binName, func(t *testing.T) {
		for _, f := range tests {
			f := f // capture range variable
			fName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
			portStart++
			data := bin{
				path: path,
				port: portStart,
			}
			t.Run(fName, func(t *testing.T) {
				t.Parallel()
				f(t, data)
			})
		}
	})
}

func testLanding(t *testing.T, data bin) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Run exporter.
	cmd := exec.CommandContext(
		ctx,
		data.path,
		"--web.listen-address", fmt.Sprintf(":%d", data.port),
		"--config.my-cnf=test_exporter.cnf",
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()

	// Get the main page.
	urlToGet := fmt.Sprintf("http://127.0.0.1:%d", data.port)
	body, err := waitForBody(urlToGet)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)

	expected := `<html lang="en">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>MySQLd Exporter</title>
    <style>body {
  font-family: -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica Neue,Arial,Noto Sans,Liberation Sans,sans-serif,Apple Color Emoji,Segoe UI Emoji,Segoe UI Symbol,Noto Color Emoji;
  margin: 0;
}
header {
  background-color: #e6522c;
  color: #fff;
  font-size: 1rem;
  padding: 1rem;
}
main {
  padding: 1rem;
}
label {
  display: inline-block;
  width: 0.5em;
}

</style>
  </head>
  <body>
    <header>
      <h1>MySQLd Exporter</h1>
    </header>
    <main>
      <h2>Prometheus Exporter for MySQL servers</h2>
      <div>Version: (version=gotest-version, branch=gotest-branch, revision=gotest-revision)</div>
      <div>
        <ul>
          
          <li><a href="/metrics">Metrics</a></li>
          
        </ul>
      </div>
      
      
    </main>
  </body>
</html>
`
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("expected != got \n%v\n", diff)
	}
}

func testProbe(t *testing.T, data bin) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Run exporter.
	cmd := exec.CommandContext(
		ctx,
		data.path,
		"--web.listen-address", fmt.Sprintf(":%d", data.port),
		"--config.my-cnf=test_exporter.cnf",
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()

	// Get the main page.
	urlToGet := fmt.Sprintf("http://127.0.0.1:%d/probe", data.port)
	body, err := waitForBody(urlToGet)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(body))

	expected := `target is required`

	if got != expected {
		t.Fatalf("got '%s' but expected '%s'", got, expected)
	}
}

// waitForBody is a helper function which makes http calls until http server is up
// and then returns body of the successful call.
func waitForBody(urlToGet string) (body []byte, err error) {
	tries := 60

	// Get data, but we need to wait a bit for http server.
	for i := 0; i <= tries; i++ {
		// Try to get web page.
		body, err = getBody(urlToGet)
		if err == nil {
			return body, err
		}

		// If there is a syscall.ECONNREFUSED error (web server not available) then retry.
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

		// There was an error, and it wasn't syscall.ECONNREFUSED.
		return nil, err
	}

	return nil, fmt.Errorf("failed to GET %s after %d tries: %s", urlToGet, tries, err)
}

// getBody is a helper function which retrieves http body from given address.
func getBody(urlToGet string) ([]byte, error) {
	resp, err := http.Get(urlToGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func Test_filterScrapers(t *testing.T) {
	type args struct {
		scrapers      []collector.Scraper
		collectParams []string
	}
	tests := []struct {
		name string
		args args
		want []collector.Scraper
	}{
		{"args_appears_in_collector",
			args{
				[]collector.Scraper{collector.ScrapeGlobalStatus{}},
				[]string{collector.ScrapeGlobalStatus{}.Name()},
			},
			[]collector.Scraper{
				collector.ScrapeGlobalStatus{},
			}},
		{"args_absent_in_collector",
			args{
				[]collector.Scraper{collector.ScrapeGlobalStatus{}},
				[]string{collector.ScrapeGlobalVariables{}.Name()},
			},
			[]collector.Scraper{collector.ScrapeGlobalStatus{}}},
		{"respect_params",
			args{
				[]collector.Scraper{
					collector.ScrapeGlobalStatus{},
					collector.ScrapeGlobalVariables{},
				},
				[]string{collector.ScrapeGlobalStatus{}.Name()},
			},
			[]collector.Scraper{
				collector.ScrapeGlobalStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterScrapers(tt.args.scrapers, tt.args.collectParams); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterScrapers() = %v, want %v", got, tt.want)
			}
		})
	}
}
