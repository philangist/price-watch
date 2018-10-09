FROM golang:1.10

RUN mkdir -p /go/src/github.com/philangist/price-watch
WORKDIR /go/src/github.com/philangist/price-watch

ADD . /go/src/github.com/philangist/price-watch

RUN go get github.com/lib/pq
