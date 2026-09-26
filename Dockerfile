FROM node:22-alpine AS web
WORKDIR /src/web/miniapp
COPY web/miniapp/package*.json ./
RUN npm ci
COPY web/miniapp/ ./
RUN npm run build

FROM node:22-alpine AS admin
WORKDIR /src/web/admin
COPY web/admin/package*.json ./
RUN npm ci
COPY web/admin/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /server ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY certs/russian_trusted_root_ca_pem.crt /usr/local/share/ca-certificates/russian_trusted_root_ca.crt
COPY certs/russian_trusted_sub_ca_pem.crt /usr/local/share/ca-certificates/russian_trusted_sub_ca.crt
RUN update-ca-certificates
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=backend /server /app/server
COPY --from=web /src/web/miniapp/dist /app/static/app
COPY --from=admin /src/web/admin/dist /app/static/admin
COPY migrations /app/migrations
USER app
EXPOSE 8080
CMD ["/bin/sh", "-c", "/app/server migrate && exec /app/server"]
