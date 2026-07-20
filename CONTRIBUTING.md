# Contributing

Prometheus uses GitHub to manage reviews of pull requests.

* If you have a trivial fix or improvement, go ahead and create a pull request,
  addressing (with `@...`) the maintainer of this repository (see
  [MAINTAINERS.md](MAINTAINERS.md)) in the description of the pull request.

* If you plan to do something more involved, first discuss your ideas
  on our [mailing list](https://groups.google.com/forum/?fromgroups#!forum/prometheus-developers).
  This will avoid unnecessary work and surely give you and us a good deal
  of inspiration.

* Relevant coding style guidelines are the [Go Code Review
  Comments](https://code.google.com/p/go-wiki/wiki/CodeReviewComments)
  and the _Formatting and style_ section of Peter Bourgon's [Go: Best
  Practices for Production
  Environments](http://peter.bourgon.org/go-in-production/#formatting-and-style).


## Local setup

The easiest way to get a local MySQL instance for development is Docker Compose.
A minimal `docker-compose.yml` is provided in the repository root.

```
docker compose up -d
# wait until MySQL accepts connections (healthcheck runs mysqladmin ping)
make
make test
```

MySQL will be available on `127.0.0.1:3306` with an empty root password.
Override the image if needed, for example:

```
MYSQL_IMAGE=mysql:5.7 docker compose up -d
```

You can also point the exporter at any existing MySQL/MariaDB instance; Compose
is only a convenience for contributors who do not already have one running.
