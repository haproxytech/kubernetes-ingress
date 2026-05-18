FROM golang:1.26-alpine AS builder
ENV GOTOOLCHAIN=auto

COPY *.go /src/
COPY go.mod /src/go.mod
COPY go.sum /src/go.sum

RUN cd /src && go build -o proxy-protocol

FROM alpine:3
WORKDIR /app
COPY --from=builder /src/proxy-protocol .

ENTRYPOINT ["./proxy-protocol"]
CMD []
