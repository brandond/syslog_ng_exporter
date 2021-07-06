FROM golang:alpine AS builder
LABEL maintainer="Jabes Pauya <jabpau93@gmail.com>"

RUN apk --no-cache upgrade
RUN apk --no-cache add alpine-sdk

ENV GO1111MODULE=on

WORKDIR /syslog_ng_exporter

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build

RUN chmod +x syslog_ng_exporter

EXPOSE 9577

# add the endpoint for the bin as /usr/local/bin/syslog_ng_exporter
ENTRYPOINT ["/usr/local/bin/syslog_ng_exporter"]

CMD [ "--help" ]