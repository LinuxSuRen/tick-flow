# syntax=docker/dockerfile:1

# Web build: compiled into the Go binary via embed.
FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tick-flow ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/tick-flow /usr/local/bin/tick-flow
ENV TICKFLOW_UPLOAD_DIR=/data/uploads
VOLUME ["/data"]
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/usr/local/bin/tick-flow"]
