CREATE USER IF NOT EXISTS 'exporter'@'%' IDENTIFIED BY 'exporter';
-- Force the plugin in case the user existed from a previous run with a different plugin
ALTER USER 'exporter'@'%' IDENTIFIED WITH caching_sha2_password BY 'exporter';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'exporter'@'%';

-- App user for seeding
CREATE USER IF NOT EXISTS 'app'@'%' IDENTIFIED BY 'app';
CREATE DATABASE IF NOT EXISTS testdb;
GRANT ALL PRIVILEGES ON testdb.* TO 'app'@'%';

FLUSH PRIVILEGES;
