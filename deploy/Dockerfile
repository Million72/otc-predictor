
# Build from repo root: docker build -f deploy/Dockerfile -t otc-predictor .
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/otc-predictor cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/otc-predictor .
# config.yaml is read from the working directory at runtime, and the
# dashboard is served from web/ — both need to ship in the image.
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/web ./web
EXPOSE 8080
CMD ["./otc-predictor"]
