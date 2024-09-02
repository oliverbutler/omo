# Build stage
FROM --platform=linux/amd64 golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .

# Run stage
FROM --platform=linux/amd64 alpine:3.14
WORKDIR /app
COPY --from=builder /app/main .
COPY static ./static
COPY templates ./templates
EXPOSE 6900
CMD ["./main"]
