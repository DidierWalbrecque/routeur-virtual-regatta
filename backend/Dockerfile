# Étape 1 : compilation de l'application.
FROM golang:1.26.2-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/vr-router-api \
    ./cmd/api

# Étape 2 : image légère utilisée pour exécuter l'application.
FROM alpine:3.23.5

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app

COPY --from=build /out/vr-router-api /usr/local/bin/vr-router-api

USER app

EXPOSE 8080

ENTRYPOINT ["vr-router-api"]