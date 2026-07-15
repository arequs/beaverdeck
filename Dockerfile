# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM node:22-alpine AS webbuild
WORKDIR /web
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26.5-alpine AS build
WORKDIR /src

ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=webbuild /cmd/server/web/dist ./cmd/server/web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/beaverdeck ./cmd/server

FROM --platform=$TARGETPLATFORM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/beaverdeck /app/beaverdeck
ENV LISTEN_ADDR=:8080 \
    DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["/app/beaverdeck"]
