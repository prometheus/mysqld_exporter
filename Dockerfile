FROM debian

COPY . /src

RUN apt-get update \
	&& apt-get install -y curl make git mercurial \ 
	&& cd /src \
	&& make \
	&& apt-get purge -y curl make git mercurial \
	&& apt-get autoremove -y \
	&& rm -rf /var/lib/apt/lists/* \
	&& rm -rf /src/.build

EXPOSE 9104

CMD ["/src/mysqld_exporter"]
