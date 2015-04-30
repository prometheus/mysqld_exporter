FROM debian

ADD mysqld_exporter .

EXPOSE 9104

CMD ./mysqld_exporter
