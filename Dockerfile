# Build frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# Build backend
FROM golang:1.26.0-alpine AS backend-builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o docker-log-viewer cmd/viewer/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=backend-builder /app/docker-log-viewer .
COPY --from=frontend-builder /app/web/dist ./web/dist

EXPOSE 9000

CMD ["./docker-log-viewer"]
