# mysqld_exporter

Exporter for MySQL server metrics http://prometheus.io/

## Building and running

    make
    ./mysqld_exporter <flags>

## Configuration
The configuration is in JSON. An example with all possible options:
```
{
   "config" : {
      "mysql_connection" : "login:password@host/dbname"
   }
}
```
Name     | Description
---------|------------
login    | Login name for connect with server
password | Password for connect with server
host     | Server hostname or ip. May be omitted
dbname   | Name of database

