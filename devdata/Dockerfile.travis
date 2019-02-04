FROM golang:1.11-stretch

COPY devdata /devdata

COPY cmd     /go/src/github.com/cloudflare/ct-log/cmd
COPY config  /go/src/github.com/cloudflare/ct-log/config
COPY ct      /go/src/github.com/cloudflare/ct-log/ct
COPY custom  /go/src/github.com/cloudflare/ct-log/custom
COPY vendor  /go/src/github.com/cloudflare/ct-log/vendor

RUN GOPATH=/go go install -race github.com/cloudflare/ct-log/cmd/server
