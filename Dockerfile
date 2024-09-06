# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Tailwind CSS stage
FROM alpine:3.14 AS tailwind
WORKDIR /app
COPY static ./static
RUN wget -O tailwindcss https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 \
    && chmod +x tailwindcss
RUN ./tailwindcss -i ./static/input.css -o ./static/output.css --minify

# Run stage
FROM alpine:3.14
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=tailwind /app/static/output.css ./static/output.css
COPY static ./static
COPY migrations ./migrations
EXPOSE 6900
CMD ["./main"]
