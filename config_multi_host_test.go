package main

import (
    "testing"
    "gopkg.in/yaml.v2"

    "github.com/smartystreets/goconvey/convey"
)

func TestValidate(t *testing.T) {
    const (
        missingName = `
            clients:
            - user: blah
              password: abc123
              port: 3306
            `
         missingUser = `
            clients:
            - name: default_client
              password: abc123
              port: 3306
            `
         missingPassword = `
            clients:
            - name: default_client
              user: blah
              port: 3306
            `
        missingPort = `
            clients:
            - name: default_client
              user: blah
              password: abc123
            `
    )
    convey.Convey("Various multi-host exporter config validation", t, func() {
        convey.Convey("Client without name", func() {
            c := &multiHostExporterConfig{}
            err :=  yaml.Unmarshal([]byte(missingName), &c)
            if err != nil {
                t.Error(err)
            }
            err = c.validate()
            convey.So(err, convey.ShouldResemble, errNameIsNotSet)
        })
        convey.Convey("Client without user", func() {
            c := &multiHostExporterConfig{}
            err :=  yaml.Unmarshal([]byte(missingUser), &c)
            if err != nil {
                t.Error(err)
            }
            err = c.validate()
            convey.So(err, convey.ShouldResemble, errUserIsNotSet)
        })
        convey.Convey("Client without password", func() {
            c := &multiHostExporterConfig{}
            err :=  yaml.Unmarshal([]byte(missingPassword), &c)
            if err != nil {
                t.Error(err)
            }
            err = c.validate()
            convey.So(err, convey.ShouldResemble, errPasswordIsNotSet)
        })
        convey.Convey("Client without port", func() {
            c := &multiHostExporterConfig{}
            err :=  yaml.Unmarshal([]byte(missingPort), &c)
            if err != nil {
                t.Error(err)
            }
            err = c.validate()
            convey.So(err, convey.ShouldResemble, errPortIsNotSet)
        })

    })
}


func TestFormDSN(t *testing.T) {
    const (
        workingClient = `
            clients:
            - name: default_client
              user: default_user
              password: default_pass
              port: 3306
            - name: rds.example.com
              user: rds_user
              password: rds_pass
              port: 8000
        `
    )
    convey.Convey("Multi Host exporter dsn", t, func() {
        convey.Convey("Default Client", func() {
            c := &multiHostExporterConfig{}
            err :=  yaml.Unmarshal([]byte(workingClient), &c)
            if err != nil {
                t.Error(err)
            }
            dsn, _ := c.formDSN("rds2.example.com")
            convey.So(dsn, convey.ShouldEqual, "default_user:default_pass@tcp(rds2.example.com:3306)/")
        })
        convey.Convey("Host specific Client", func() {
            c := &multiHostExporterConfig{}
            err :=  yaml.Unmarshal([]byte(workingClient), &c)
            if err != nil {
                t.Error(err)
            }
            dsn, _ := c.formDSN("rds.example.com")
            convey.So(dsn, convey.ShouldEqual, "rds_user:rds_pass@tcp(rds.example.com:8000)/")
        })

    })
}
