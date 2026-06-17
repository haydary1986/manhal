# ---- build ----
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/manhal ./cmd/server

# ---- run ----
FROM alpine:3.20
# ca-certificates/tzdata for TLS + Baghdad time; poppler-utils + tesseract (with
# Arabic & English data) power OCR of scanned PDFs in the PDF→Word converter.
RUN apk add --no-cache ca-certificates tzdata poppler-utils tesseract-ocr tesseract-ocr-data-ara tesseract-ocr-data-eng
WORKDIR /app
COPY --from=build /out/manhal /app/manhal
# Seed data lives in /app/seed; /app/data is a writable volume seeded on first
# run by the entrypoint, so admin edits persist across redeploys.
COPY data /app/seed
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh
ENV DATA_DIR=/app/data
# Admin web server (enabled only when ADMIN_WEB_TOKEN is set).
EXPOSE 8080
ENTRYPOINT ["/app/docker-entrypoint.sh"]
