# Stage 1: Build binaries
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/udayana-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/udayana-migrate ./cmd/migrate

# Stage 2: Minimal Production Image
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/bin/udayana-api /app/udayana-api
COPY --from=builder /app/bin/udayana-migrate /app/udayana-migrate

EXPOSE 8080

CMD ["/app/udayana-api"]
