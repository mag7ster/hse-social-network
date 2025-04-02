FROM golang:1.24-alpine

WORKDIR /hse-social-network
COPY go.* .
COPY proto/ ./proto/

RUN go mod download -x

WORKDIR /hse-social-network/api-gateway
COPY api-gateway/*.go .

RUN go build .

ENTRYPOINT ["./api-gateway"]
