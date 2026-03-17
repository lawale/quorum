FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/quorum ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S quorum && adduser -S quorum -G quorum

COPY --from=builder /bin/quorum /bin/quorum
COPY --from=builder /app/migrations /migrations

USER quorum

EXPOSE 8080

ENTRYPOINT ["/bin/quorum"]
CMD ["-config", "/etc/quorum/config.yaml"]
