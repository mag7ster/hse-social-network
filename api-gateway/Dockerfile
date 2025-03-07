FROM golang:1.24-alpine

WORKDIR /hse-social-network
COPY go.* .

RUN go mod download -x

WORKDIR /hse-social-network/api-gateway
COPY api-gateway/main.go .

RUN go build .

ENTRYPOINT ["./api-gateway"]
