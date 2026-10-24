# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY internal ./internal
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/colday2026 .
RUN mkdir -p /out/data /out/uploads

FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /colday_app
COPY --from=builder /out/colday2026 ./colday2026
COPY static ./static
COPY --from=builder --chown=65532:65532 /out/data ./data
COPY --from=builder --chown=65532:65532 /out/uploads ./static/uploads
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/colday_app/colday2026"]



