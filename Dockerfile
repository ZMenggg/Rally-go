# Stage 1: Build
FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /rally ./cmd/rally/

# Stage 2: Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /rally /usr/local/bin/rally
EXPOSE 1080
ENTRYPOINT ["rally"]
CMD ["run"]
