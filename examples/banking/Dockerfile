# Build from the repository root:
#   docker build -f examples/banking/Dockerfile .
# The example depends on the Quorum Go client in this repository via a
# replace directive, so the whole module tree is the build context.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
COPY examples/banking/go.mod examples/banking/go.sum ./examples/banking/
RUN cd examples/banking && go mod download

COPY . .
RUN cd examples/banking && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /bin/banking .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && \
    addgroup -S quorum && adduser -S quorum -G quorum
COPY --from=builder /bin/banking /bin/banking
COPY --from=builder /src/examples/banking/templates /templates

USER quorum

WORKDIR /
EXPOSE 3001

CMD ["/bin/banking"]
