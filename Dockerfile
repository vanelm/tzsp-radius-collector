# syntax=docker/dockerfile:1

FROM golang:1.21-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ENV CGO_ENABLED=0 GOOS=linux GOMAXPROCS=1
RUN go build -trimpath -ldflags="-s -w" -p 1 -o /out/tzsp-radius-collector ./main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /out/tzsp-radius-collector /app/tzsp-radius-collector
COPY config/dictionary /app/config/dictionary

ENV URSA_TZSP_DICTIONARY_GLOB=/app/config/dictionary/dictionary.*
EXPOSE 8098 37008/udp

ENTRYPOINT ["/app/tzsp-radius-collector"]
