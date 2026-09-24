# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ledsured ./cmd/ledsured

FROM alpine:3.24
RUN apk add --no-cache ca-certificates wget \
    && addgroup -S ledsure && adduser -S ledsure -G ledsure
COPY --from=builder /out/ledsured /usr/local/bin/ledsured
USER ledsure

EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=5 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/ledsured"]
