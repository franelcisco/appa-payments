# syntax=docker/dockerfile:1

# ====== Build stage ======
FROM golang:1.25 AS build
WORKDIR /app

# 1) Dependencias (cache-friendly)
COPY go.mod go.sum ./
RUN go mod download

# 2) Código
COPY . .

# 3) Compila tu main de cmd/main.go
#    Si tu paquete principal está en cmd/, este comando es correcto.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags "-s -w" -o /server ./cmd

# ====== Runtime stage ======
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /server /app/server

# Cloud Run: el contenedor debe escuchar en $PORT (por defecto 8080)
ENV PORT=8080
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
