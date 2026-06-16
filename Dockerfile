# ---- build ----
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/manhal ./cmd/server

# ---- run ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/manhal /app/manhal
COPY data /app/data
ENV DATA_DIR=/app/data
# Admin web server (enabled only when ADMIN_WEB_TOKEN is set).
EXPOSE 8080
ENTRYPOINT ["/app/manhal"]
