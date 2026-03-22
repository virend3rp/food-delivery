FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /delivery-service ./cmd/delivery-service

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /delivery-service .
CMD ["./delivery-service"]
