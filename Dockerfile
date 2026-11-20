FROM public.ecr.aws/docker/library/golang:1.24-alpine

WORKDIR /app

ENV GOPROXY=https://proxy.golang.org,direct
ENV GOTOOLCHAIN=local

RUN apk add --no-cache ca-certificates tzdata git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/udayana-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/udayana-migrate ./cmd/migrate

EXPOSE 8080

CMD ["/app/udayana-api"]
