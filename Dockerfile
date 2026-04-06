FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o control-plane ./cmd/control-plane

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /app/control-plane /usr/local/bin/
CMD ["control-plane"]
