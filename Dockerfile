# Stage 1: Build
FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download 2>&1 || true
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /rally ./cmd/rally/

# Stage 2: Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /rally /usr/local/bin/rally
EXPOSE 1080 9090
ENTRYPOINT ["rally"]
CMD ["run"]
