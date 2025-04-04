# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN ./tailwindcss -i ./static/input.css -o ./static/output.css --minify

RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Run stage
FROM alpine:3.14

# Need curl for healthcheck
RUN apk add --no-cache curl

WORKDIR /app
COPY --from=builder /app/main .
COPY static ./static


# Copy the output.css file
COPY --from=builder /app/static/output.css ./static/output.css
COPY migrations ./migrations
EXPOSE 6900
CMD ["./main"]
