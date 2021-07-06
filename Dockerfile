FROM golang:alpine AS builder
LABEL maintainer="Jabes Pauya <jabpau93@gmail.com>"

RUN apk --no-cache upgrade
RUN apk --no-cache add alpine-sdk

ENV GO1111MODULE=on 

WORKDIR /syslog_ng_exporter/

# Copy go mod and sum files
COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/syslog_ng_exporter

FROM scratch

COPY --from=build /bin/syslog_ng_exporter /bin/syslog_ng_exporter

EXPOSE 9577

ENTRYPOINT ["/bin/syslog_ng_exporter"]