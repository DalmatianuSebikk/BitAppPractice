# syntax=docker/dockerfile:1

# ── stage 1: build ─────────────────────────────────────────────────────
FROM golang:1.25.6-alpine AS builder

WORKDIR /src

# leverage layer cache — pull deps before copying source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# pure-Go build — modernc.org/sqlite has no CGO, so CGO_ENABLED=0 is fine
# and produces a statically-linked binary.
ENV CGO_ENABLED=0
ENV GOOS=linux

RUN go build -trimpath -ldflags="-s -w" -o /out/practicabitapp .

# ── stage 2: runtime ───────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates: outbound HTTPS for SSRF and webhook-XSS chapters.
# iputils:         /bin/ping for the cmdi chapter (busybox ping works for
#                  unprivileged ICMP on modern kernels; iputils makes it
#                  reliable across host kernels).
RUN apk add --no-cache ca-certificates iputils

WORKDIR /app
COPY --from=builder /out/practicabitapp /app/practicabitapp

EXPOSE 9998

# Default healthcheck (compose can override): GET / and expect 200.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -q --spider http://127.0.0.1:9998/ || exit 1

ENTRYPOINT ["/app/practicabitapp"]
