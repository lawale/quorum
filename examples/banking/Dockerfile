FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder
ARG TARGETOS TARGETARCH

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /bin/banking .

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && \
    addgroup -S quorum && adduser -S quorum -G quorum
COPY --from=builder /bin/banking /bin/banking
COPY --from=builder /app/templates /templates

USER quorum

WORKDIR /
EXPOSE 3001

CMD ["/bin/banking"]
