# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/tzsp-radius-collector ./main.go

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=builder /out/tzsp-radius-collector /app/tzsp-radius-collector
COPY config/dictionary /app/config/dictionary

ENV URSA_TZSP_DICTIONARY_GLOB=/app/config/dictionary/dictionary.*
EXPOSE 8098 37008/udp

ENTRYPOINT ["/app/tzsp-radius-collector"]
