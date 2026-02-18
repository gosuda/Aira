# ============================================================================
# Stage 1: Build SvelteKit frontend
# ============================================================================
FROM node:22-alpine AS builder-web

RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /src/web

# Install dependencies first (layer cache optimization).
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml web/.npmrc ./
RUN pnpm install --frozen-lockfile

# Copy source and build.
COPY web/ ./
RUN pnpm run build

# ============================================================================
# Stage 2: Build Go binary
# ============================================================================
FROM golang:1.26-alpine AS builder-go

WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

# Copy full source tree.
COPY . .

# Inject the frontend build output into web/build/ for go:embed.
COPY --from=builder-web /src/web/build/ ./web/build/

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /aira ./cmd/aira

# ============================================================================
# Stage 3: Minimal runtime image
# ============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="aira" \
      org.opencontainers.image.description="Aira - AI agent orchestration platform" \
      org.opencontainers.image.source="https://github.com/gosuda/aira" \
      org.opencontainers.image.vendor="gosuda" \
      org.opencontainers.image.licenses="MIT"

COPY --from=builder-go /aira /aira

EXPOSE 8080

ENTRYPOINT ["/aira"]
