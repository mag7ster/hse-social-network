FROM golang:1.24-alpine

WORKDIR /hse-social-network
COPY go.* .

RUN go mod download -x

WORKDIR /hse-social-network/user-service
COPY user-service/*.go .

RUN go build .

ENTRYPOINT ["./user-service"]
