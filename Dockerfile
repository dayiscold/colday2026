FROM golang:1.26-alpine AS builder
LABEL authors="malid_gafi"

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/colday2026 .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /colday_app
COPY --from=builder /out/colday2026 /colday_app/
COPY static ./static
RUN adduser -D gafi && mkdir -p /colday_app/static/uploads /colday_app/data \
    && chown -R gafi:gafi /colday_app
USER gafi

EXPOSE 8080
ENTRYPOINT ["/colday_app/colday2026"]



