FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/banking .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/banking /bin/banking
COPY --from=builder /app/templates /templates

WORKDIR /
EXPOSE 3001

CMD ["/bin/banking"]
