FROM golang:1.25-alpine AS builder

COPY *.go /src/
COPY go.mod /src/go.mod

RUN cd /src && go build -o echo-http

FROM alpine:3
RUN apk --no-cache add openssl
WORKDIR /app
COPY --from=builder /src/echo-http .
COPY generate-cert.sh .
RUN chmod +x generate-cert.sh

ENTRYPOINT ["./echo-http"]
CMD []
