# syntax=docker/dockerfile:1
FROM node:22-alpine AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS go-builder
RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/aigw ./cmd/aigw

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && addgroup -S aigw && adduser -S -G aigw aigw
WORKDIR /app
COPY --from=go-builder /out/aigw /usr/local/bin/aigw
COPY --from=web-builder /src/web/build /app/web/build
COPY configs/config.example.yml /app/configs/config.example.yml
USER aigw
EXPOSE 8080
ENV AIGW_CONFIG=/app/configs/config.yml
ENTRYPOINT ["/usr/local/bin/aigw"]
