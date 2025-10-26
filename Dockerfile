FROM golang:1.24.7-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o docker-log-viewer cmd/viewer/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/docker-log-viewer .
COPY --from=builder /app/web ./web

EXPOSE 9000

CMD ["./docker-log-viewer"]
