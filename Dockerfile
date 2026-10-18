# ebb-web: the hosted web app. It holds no data and no secrets; the page runs in
# the visitor's browser and talks to ebb on that visitor's machine.
#   docker build -t ebb-web .
# Base images are pinned by digest (Dependabot moves them); bun matches CI.

FROM oven/bun:1.4.3-alpine@sha256:629e17411f1f129dbec3af78d5af9c9f2a937435c80349437206c6b0b7422373 AS ui
WORKDIR /src
COPY ui/package.json ui/bun.lock ./ui/
RUN cd ui && bun install --frozen-lockfile
COPY ui ./ui
# vite.config.ts reads this marker to restore it after emptying the output directory.
COPY internal/server/assets/dist/README ./internal/server/assets/dist/README
RUN cd ui && bun run build

FROM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /src/internal/server/assets/dist ./internal/server/assets/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/ebb-web ./cmd/ebb-web

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/ebb-web /ebb-web
ENV EBB_WEB_LISTEN=0.0.0.0:8080
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/ebb-web", "--health-check"]
ENTRYPOINT ["/ebb-web"]
