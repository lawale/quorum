FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/maker-checker ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/maker-checker /bin/maker-checker
COPY --from=builder /app/migrations /migrations

EXPOSE 8080

ENTRYPOINT ["/bin/maker-checker"]
CMD ["-config", "/etc/maker-checker/config.yaml"]
