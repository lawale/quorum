FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETOS TARGETARCH

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

RUN addgroup -S quorum && adduser -S quorum -G quorum

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /bin/quorum ./cmd/server

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /bin/quorum /bin/quorum
COPY --from=builder /app/migrations /migrations

USER quorum

EXPOSE 8080

ENTRYPOINT ["/bin/quorum"]
CMD ["-config", "/etc/quorum/config.yaml"]
