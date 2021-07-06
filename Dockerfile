FROM golang:alpine AS builder

LABEL maintainer="Jabes Pauya <jabpau93@gmail.com>"

ENV GO1111MODULE=on 

WORKDIR /app

# Copy go mod and sum files
COPY go.mod .

COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build 

FROM scratch

COPY --from=builder /app/syslog_ng_exporter /app/

EXPOSE 9577

ENTRYPOINT ["/app/syslog_ng_exporter"]