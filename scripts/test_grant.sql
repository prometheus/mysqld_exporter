CREATE USER 'exporter'@'localhost' IDENTIFIED BY 'integration-test' WITH MAX_USER_CONNECTIONS 3;
GRANT PROCESS, REPLICATION CLIENT TO 'exporter'@'localhost';
GRANT SELECT ON performance_schema.* TO 'exporter'@'localhost';
GRANT SELECT ON information_schema.* TO 'exporter'@'localhost';

