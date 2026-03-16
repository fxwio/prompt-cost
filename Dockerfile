FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /bin/prompt-cost ./cmd/server/main.go

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget tzdata
WORKDIR /app
COPY --from=builder /bin/prompt-cost .
EXPOSE 8092
ENTRYPOINT ["/app/prompt-cost"]
