FROM golang:1.24-alpine

WORKDIR /hse-social-network
COPY go.* .
COPY proto/ ./proto/

RUN go mod download -x

WORKDIR /hse-social-network/posts-comments-service
COPY posts-comments-service/*.go .

RUN go build .

ENTRYPOINT ["./posts-comments-service"]
