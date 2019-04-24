# Syslog-NG Exporter for Prometheus

Exports syslog-ng statistics via HTTP for Prometheus consumption.

Help on flags:

```
  --help                        Show context-sensitive help (also try --help-long and --help-man).
  --version                     Print version information.
  --telemetry.address=":9577"   Address on which to expose metrics.
  --telemetry.endpoint="/metrics"
                                Path under which to expose metrics.
  --socket.path="/var/lib/syslog-ng/syslog-ng.ctl"
                                Path to syslog-ng control socket.
  --log.level="info"            Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]
  --log.format="logger:stderr"  Set the log target and format. Example: "logger:syslog?appname=bob&local=7" or "logger:stdout?json=true"
```

Tested with syslog-ng 3.5.6


# Using Docker

## Build the compatible binary
To make sure that exporter binary created by build job is suitable to run on busybox environment, generate the binary using Makefile definition. Inside project directory run:
```
make
```
*Please be aware that binary generated using `go get` or `go build` with defaults may not work in busybox/alpine base images.*

## Build image

Run the following commands from the project root directory.

```
docker build -t apache_exporter .
```

## Run

```
docker run -d -p 9577:9577 -v /var/lib/syslog-ng/syslog-ng.ctl:/syslog-ng.ctl \
  syslog_ng_exporter --socket.path=/syslog-ng.ctl

```

## Collectors

```
# HELP syslog_ng_destination_messages_dropped_total Number of messages dropped by this destination.
# TYPE syslog_ng_destination_messages_dropped_total counter
# HELP syslog_ng_destination_messages_processed_total Number of messages processed by this destination.
# TYPE syslog_ng_destination_messages_processed_total counter
# HELP syslog_ng_destination_messages_stored_total Number of messages stored by this destination.
# TYPE syslog_ng_destination_messages_stored_total gauge
# HELP syslog_ng_source_messages_processed_total Number of messages processed by this source.
# TYPE syslog_ng_source_messages_processed_total counter
# HELP syslog_ng_up Reads 1 if the syslog-ng server could be reached, else 0.
# TYPE syslog_ng_up gauge
```

## Author

The exporter was originally created by [brandond](https://github.com/brandond), heavily inspired by the [apache_exporter](https://github.com/Lusitaniae/apache_exporter/).
