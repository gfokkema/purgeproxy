FROM golang:1.11 as builder

WORKDIR /build
COPY . /build

ENV GO111MODULE=on
RUN CGO_ENABLED=0 GOOS=linux go build -o purge-proxy

FROM alpine:latest

WORKDIR /root/
RUN apk --no-cache add ca-certificates

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/purge-proxy .

ENV KUBECLUSTER=1
CMD ["./purge-proxy"]
