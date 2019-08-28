FROM golang:alpine AS builder
RUN apk --no-cache upgrade
RUN apk --no-cache add alpine-sdk
COPY ./ /go/src/github.com/brandond/syslog_ng_exporter/
WORKDIR /go/src/github.com/brandond/syslog_ng_exporter/
RUN make

FROM quay.io/prometheus/busybox:latest
LABEL maintainer="Brad Davidson <brad@oatmail.org>"
COPY --from=builder /go/src/github.com/brandond/syslog_ng_exporter/syslog_ng_exporter /bin/syslog_ng_exporter
ENTRYPOINT ["/bin/syslog_ng_exporter"]
EXPOSE     9577
