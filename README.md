# frontend

The HTTP frontend for the platform-demo e-commerce platform. It serves the web UI, orchestrates calls to all downstream gRPC microservices, and renders HTML templates. Part of a broader microservices platform built with full observability, GitOps, and internal developer platform tooling.

## Overview

The frontend is an HTTP server that exposes the following routes:

| Route | Method | Description |
|---|---|---|
| `/` | GET | Home page — product listing |
| `/product/{id}` | GET | Product detail page |
| `/cart` | GET | View cart |
| `/cart` | POST | Add item to cart |
| `/cart/empty` | POST | Empty cart |
| `/cart/checkout` | POST | Place order |
| `/setCurrency` | POST | Change display currency |
| `/logout` | GET | Clear session cookies |
| `/product-meta/{ids}` | GET | Product metadata (JSON) |
| `/static/` | GET | Static assets |
| `/_healthz` | GET | Health check |

**Port:** `8080` (HTTP)  
**Metrics Port:** `9464` (Prometheus)  
**Protocol:** HTTP  
**Language:** Go  
**Called by:** User (browser) / loadgenerator

## Requirements

- Go 1.21+
- Docker
- All downstream services running (see dependencies below)

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `PRODUCT_CATALOG_SERVICE_ADDR` | Yes | e.g. `product-catalog-service:3550` |
| `CURRENCY_SERVICE_ADDR` | Yes | e.g. `currency-service:7000` |
| `CART_SERVICE_ADDR` | Yes | e.g. `cart-service:7070` |
| `RECOMMENDATION_SERVICE_ADDR` | Yes | e.g. `recommendation-service:8080` |
| `CHECKOUT_SERVICE_ADDR` | Yes | e.g. `checkout-service:5050` |
| `SHIPPING_SERVICE_ADDR` | Yes | e.g. `shipping-service:50051` |
| `AD_SERVICE_ADDR` | Yes | e.g. `ad-service:9555` |
| `PORT` | No | HTTP server port (default: `8080`) |
| `METRICS_PORT` | No | Prometheus metrics port (default: `9464`) |
| `ENABLE_TRACING` | No | Set to `1` to enable OTel tracing |
| `COLLECTOR_SERVICE_ADDR` | No | OTLP gRPC collector e.g. `alloy:4317` (required if tracing enabled) |
| `ENABLE_PROFILING` | No | Set to `1` to enable Pyroscope profiling |
| `PYROSCOPE_ADDR` | No | Pyroscope endpoint (default: `http://pyroscope:4040`) |
| `OTEL_SERVICE_NAME` | No | Service name reported to OTel (default: `frontend`) |
| `BASE_URL` | No | URL prefix if running behind a path-based proxy |
| `ENV_PLATFORM` | No | Platform label: `local`, `gcp`, `aws`, `azure`, `onprem`, `alibaba` |
| `CYMBAL_BRANDING` | No | Set to `true` to switch to Cymbal branding |
| `FRONTEND_MESSAGE` | No | Banner message shown on all pages |

## Running Locally

### 1. Build and run

```bash
go build -o frontend .
./frontend
```

### 2. Run with Docker

```bash
docker build -t frontend .

docker run -p 8080:8080 -p 9464:9464 \
  -e PRODUCT_CATALOG_SERVICE_ADDR=product-catalog-service:3550 \
  -e CURRENCY_SERVICE_ADDR=currency-service:7000 \
  -e CART_SERVICE_ADDR=cart-service:7070 \
  -e RECOMMENDATION_SERVICE_ADDR=recommendation-service:8080 \
  -e CHECKOUT_SERVICE_ADDR=checkout-service:5050 \
  -e SHIPPING_SERVICE_ADDR=shipping-service:50051 \
  -e AD_SERVICE_ADDR=ad-service:9555 \
  -e ENABLE_TRACING=1 \
  -e COLLECTOR_SERVICE_ADDR=alloy:4317 \
  -e ENABLE_PROFILING=1 \
  -e PYROSCOPE_ADDR=http://pyroscope:4040 \
  frontend
```

## Project Structure

```
├── main.go               # Entrypoint — server bootstrap, routing
├── telemetry.go          # Prometheus metrics, OTel tracing, Pyroscope profiling
├── handlers.go           # HTTP route handlers
├── rpc.go                # gRPC client calls to downstream services
├── middleware.go         # Logging middleware, session ID middleware
├── genproto/             # Generated gRPC stubs (demo.pb.go, demo_grpc.pb.go)
├── money/                # Currency arithmetic helpers
├── validator/            # Request payload validation
├── templates/            # HTML templates (home, product, cart, order, error)
├── static/               # CSS, icons, images
├── go.mod
├── go.sum
└── Dockerfile
```

## Observability

- **Traces** — OTLP gRPC → Alloy → Tempo. Both inbound HTTP spans (`otelhttp`) and outbound gRPC client spans (`otelgrpc`) are instrumented automatically.
- **Metrics** — Prometheus endpoint on `:9464/metrics`, scraped by Alloy → Mimir. Exposes `http_requests_total`, `http_request_duration_seconds`, and `http_requests_in_flight` labelled by method, route, and status code.
- **Logs** — Structured JSON logs via `logrus` to stdout, collected by Alloy via Docker socket → Loki. Every request logs path, method, request ID, session ID, response status, and duration.
- **Profiles** — Continuous CPU and allocation profiling via Pyroscope Go SDK → Pyroscope. Enabled via `ENABLE_PROFILING=1`.

## Part Of

This service is part of [platform-demo](https://github.com/mladenovskistefan111) — a full platform engineering project featuring microservices, observability (LGTM stack), GitOps (Argo CD), policy enforcement (Kyverno), infrastructure provisioning (Crossplane), and an internal developer portal (Backstage).