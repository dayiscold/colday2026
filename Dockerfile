# syntax=docker/dockerfile:1
FROM node:24-alpine AS frontend
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json vite.config.ts ./
COPY frontend ./frontend
RUN npm run build

FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY internal ./internal
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/colday2026 .
RUN mkdir -p /out/data /out/uploads

FROM gcr.io/distroless/static-debian13:nonroot AS runtime

WORKDIR /colday_app
COPY --from=builder /out/colday2026 ./colday2026
COPY --exclude=app static ./static
COPY --from=builder --chown=65532:65532 /out/data ./data
COPY --from=builder --chown=65532:65532 /out/uploads ./static/uploads
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/colday_app/colday2026"]

# Local Compose uses the frontend built on the host, avoiding npm in Docker.
FROM runtime AS local
COPY static/app ./static/app

# The default target builds everything from source for CI and release images.
FROM runtime AS production
COPY --from=frontend /src/static/app ./static/app
