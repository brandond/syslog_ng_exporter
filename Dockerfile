FROM golang:alpine AS builder
WORKDIR /go/src/github.com/brandond/syslog_ng_exporter
COPY *.go Makefile vendor /go/src/github.com/brandond/syslog_ng_exporter/
RUN apk --no-cache add curl git make
RUN make

FROM quay.io/prometheus/busybox:latest
COPY --from=builder /go/src/github.com/brandond/syslog_ng_exporter/syslog_ng_exporter /bin/syslog_ng_exporter
ENTRYPOINT ["/bin/syslog_ng_exporter"]
EXPOSE     9577
