FROM golang:1.24-alpine AS builder

WORKDIR /app

# Dipendenze Go
COPY go.mod go.sum ./
RUN go mod download

# Build app
COPY . .
RUN go build -o gazeparty .

# Immagine finale
FROM alpine:latest

# Installa ffmpeg
RUN apk add --no-cache ffmpeg

WORKDIR /app

# Copia binary e templates
COPY --from=builder /app/gazeparty .
COPY templates ./templates

EXPOSE 8066

CMD ["./gazeparty"]
