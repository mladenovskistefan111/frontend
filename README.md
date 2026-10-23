# frontend

The HTTP web storefront for the platform-demo e-commerce platform. It serves server-rendered HTML pages and static assets, keeps a session cookie per visitor, and fans out to seven backend gRPC services to build every page. It is the **only** component users (and ingress) talk to directly; everything else is internal.

Part of the [platform-demo](https://github.com/platform-boutique) platform — microservices, observability (LGTM stack), GitOps (Argo CD), policy enforcement (Kyverno), infrastructure provisioning (Crossplane) and an internal developer portal (Backstage). See the master `README.md` in the platform-demo root for the whole-system view.

## At a glance

| | |
|---|---|
| **Language / runtime** | Go 1.26 (`gorilla/mux`, `html/template`), static binary |
| **Image** | `mladenovskistefan/frontend` |
| **Protocol / port** | HTTP/1.1 on `8080` |
| **Metrics** | Pushed over OTLP/gRPC (Prometheus registry bridged into OpenTelemetry); `/metrics` on `9464` is still served for debugging |
| **Health** | `GET /_healthz` → `200 ok` — static, no dependency checks |
| **State** | None server-side; session id lives in a browser cookie, carts live in `cart-service` |
| **Depends on (gRPC)** | `product-catalog-service`, `currency-service`, `cart-service`, `recommendation-service`, `checkout-service`, `shipping-service`, `ad-service` |
| **Called by** | Browsers / ingress / load generators |
| **Not used** | `payment-service` and `email-service` are only reached via `checkout-service`. `shopping-assistant-service` is **not** wired in |

## HTTP routes

All routes are prefixed with `BASE_URL` (empty by default).

| Route | Method | Description |
|---|---|---|
| `/` | GET, HEAD | Home — product grid, currency picker, ad, cart size |
| `/product/{id}` | GET, HEAD | Product page — product, price, recommendations, ad |
| `/cart` | GET, HEAD | View cart — items, shipping estimate, total, recommendations |
| `/cart` | POST | Add item (form fields `product_id`, `quantity`) |
| `/cart/empty` | POST | Empty the cart |
| `/cart/checkout` | POST | Validate the form, call `checkout-service.PlaceOrder`, render the order page |
| `/setCurrency` | POST | Set the display currency cookie (`currency_code`) |
| `/logout` | GET | Expire all cookies and redirect to `/` |
| `/product-meta/{ids}` | GET | JSON of a single product (used by the page scripts) |
| `/static/*` | GET | CSS, icons, product images (`static/img/products/*.jpg`) |
| `/_healthz` | GET | Liveness/readiness (`ok`) |
| `/robots.txt` | GET | `Disallow: /` |

**Supported currencies** (whitelist in code, intersected with what `currency-service` offers): `USD`, `EUR`, `CAD`, `JPY`, `GBP`, `TRY`. Default is `USD`.

### Which backend each page calls

| Page | Calls |
|---|---|
| Home | currency (list + convert per product), product-catalog (list), cart (get), ad (100 ms deadline, failures ignored) |
| Product | product-catalog (get), currency, cart, recommendation → product-catalog (up to 4), ad |
| Cart | cart (get), shipping (quote with nil address), currency, product-catalog, recommendation |
| Checkout | checkout (PlaceOrder), then recommendation + currency for the confirmation page |

### Sessions & cookies

| Cookie | Purpose | Flags |
|---|---|---|
| `shop_session-id` | random UUID per visitor; used as the cart `user_id` | `HttpOnly`, **`Secure`**, `SameSite=Strict`, 48 h |
| `shop_currency` | chosen display currency | `HttpOnly`, **`Secure`**, `SameSite=Strict`, 48 h |

> **Kubernetes gotcha — cookies are `Secure`.** Browsers only store `Secure` cookies over HTTPS (or `http://localhost`). If you reach the frontend over plain HTTP at a LoadBalancer/NodePort IP or hostname, the session cookie is dropped, every request becomes a new session, and the cart appears to be empty after "Add to cart". Use `kubectl port-forward` (→ `localhost`), or terminate TLS at the ingress/gateway, or set `ENABLE_SINGLE_SHARED_SESSION=true` as a demo workaround (see below).

## Configuration

| Variable | Required | Default | Description |
|---|---|---|---|
| `PRODUCT_CATALOG_SERVICE_ADDR` | **Yes** | — | `host:port` of product-catalog (gRPC), e.g. `product-catalog-service:3550` |
| `CURRENCY_SERVICE_ADDR` | **Yes** | — | e.g. `currency-service:7000` |
| `CART_SERVICE_ADDR` | **Yes** | — | e.g. `cart-service:7070` |
| `RECOMMENDATION_SERVICE_ADDR` | **Yes** | — | e.g. `recommendation-service:8080` |
| `CHECKOUT_SERVICE_ADDR` | **Yes** | — | e.g. `checkout-service:5050` |
| `SHIPPING_SERVICE_ADDR` | **Yes** | — | e.g. `shipping-service:50051` |
| `AD_SERVICE_ADDR` | **Yes** | — | e.g. `ad-service:9555` |
| `PORT` | No | `8080` | HTTP listen port |
| `LISTEN_ADDR` | No | `` (all interfaces) | Bind address; leave empty in containers |
| `METRICS_PORT` | No | `9464` | Prometheus listener port |
| `BASE_URL` | No | `` | URL path prefix when served under a sub-path (e.g. `/shop`); all routes and static URLs move under it |
| `FRONTEND_MESSAGE` | No | — | Banner text shown on pages |
| `BANNER_COLOR` | No | — | Banner colour passed to the home template |
| `CYMBAL_BRANDING` | No | `false` | `true` switches to the Cymbal Shops branding |
| `ENABLE_SINGLE_SHARED_SESSION` | No | `false` | `true` makes every new visitor share one hard-coded session id (cart shared by everyone). Demo/test workaround only |
| `TEMPLATE_PATH` | No | `templates/*.html` | Glob of HTML templates (relative to `/app`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | No | unset (telemetry off) | OTLP/**gRPC** endpoint for **traces, metrics and logs** (scheme required), e.g. `http://alloy.monitoring.svc.cluster.local:4317`. Legacy fallback: `COLLECTOR_SERVICE_ADDR` (`host:port`). Telemetry is on iff an endpoint is set |
| `ENABLE_TRACING` | No | — | Legacy switch: `0` forces all telemetry off even if an endpoint is set (it is no longer needed to turn telemetry on) |
| `OTEL_METRIC_EXPORT_INTERVAL` | No | `15000` | Milliseconds between metric pushes (Go also accepts durations such as `15s`) |
| `OTEL_SERVICE_NAME` | No | `frontend` | `service.name` — becomes the Prometheus `job` and the Loki `service_name`; keep the default, dashboards depend on it |
| `PYROSCOPE_ADDR` | No | `http://pyroscope:4040` | Profiles use the Pyroscope SDK push protocol (**not** OTLP). Point it at Alloy (`http://alloy.monitoring.svc.cluster.local:4040`), which forwards to Pyroscope, or directly at Pyroscope |
| `ENABLE_PROFILING` | No | off | `1` starts the Pyroscope SDK |

Notes:

- All seven `*_ADDR` variables are **mandatory**: a missing one makes the process `panic` at startup. They are plain `host:port` (no scheme). Use FQDNs (`svc.<namespace>.svc.cluster.local:<port>`) if services live in other namespaces.
- gRPC connections use `grpc.NewClient` (lazy): the pod starts and `/_healthz` returns `ok` even if every backend is down. Page loads then fail with HTTP 500 until backends return.
- The earlier README listed `ENV_PLATFORM`; it is **not read by the code**.
- Log level is hard-coded to `debug` (JSON via `logrus`) — expect chatty logs; there is no env switch.
- Telemetry is **pushed**, not scraped: traces, metrics and logs share one OTLP/gRPC endpoint (Alloy); profiles go to Pyroscope (via Alloy). `service.instance.id` is the pod hostname, so replicas stay distinct in Mimir.
- A Prometheus `/metrics` endpoint is still served on `METRICS_PORT` for debugging, but nothing has to scrape it — the same registry is bridged into OTLP, so metric names are unchanged.
- Logs: the `logrus` logger keeps writing JSON to stdout and additionally mirrors every record over OTLP (with trace ids when a context is attached).

## Container image

Multi-stage [`Dockerfile`](Dockerfile):

| Stage | Base | What happens |
|---|---|---|
| builder | `golang:1.26-alpine` | `go mod download`, `go build -ldflags="-s -w" -o /bin/frontend ./cmd/server` (`CGO_ENABLED=0`; target arch = builder's arch) |
| runtime | `alpine:3.24` | binary + `templates/` + `static/` under `/app`; adds `ca-certificates` |

- **Entrypoint:** `./frontend` (working dir `/app` — templates and static files are loaded by relative path, so don't change the working dir).
- **User:** `appuser` — UID `100`, GID `101` (verified). Set `runAsUser: 100` for `runAsNonRoot`.
- **EXPOSE:** `8080`.
- **Docker `HEALTHCHECK`:** `wget -qO- http://localhost:${PORT:-8080}/_healthz`. Alpine's busybox `wget` is in the image, so an exec probe is possible, but `httpGet` is simpler.
- **Image size:** ~15 MB.
- The repo root may contain a compiled `server` binary (git-ignored) — it is not used by the image.

## Kubernetes deployment notes

**Workload:** stateless `Deployment`; scale freely (sessions are in a cookie, carts in `cart-service`; no sticky sessions required). This is the one workload that needs external exposure.

**Ports to declare**

| Name | Port | Protocol | Expose via Service? |
|---|---|---|---|
| `http` | `8080` | TCP (HTTP/1.1) | Yes — Service port `80` → `8080`; front it with an Ingress / Gateway / LoadBalancer |
| `metrics` | `9464` | TCP (HTTP/1.1) | Optional (debugging/pprof). Not scraped by default — metrics are pushed over OTLP |

**Exposure:** serve over **HTTPS** (see cookie gotcha). Ingress `Service` target is `frontend:80` → pod `8080`. If you use a sub-path ingress, also set `BASE_URL` to match. Set the NetworkPolicy so that only the ingress controller can reach `8080`.

**Probes**

```yaml
startupProbe:
  httpGet: { path: /_healthz, port: 8080 }
  periodSeconds: 2
  failureThreshold: 15
readinessProbe:
  httpGet: { path: /_healthz, port: 8080 }
  periodSeconds: 5
livenessProbe:
  httpGet: { path: /_healthz, port: 8080 }
  periodSeconds: 10
  failureThreshold: 3
```

Health is static, so readiness doesn't reflect backend health. Pod startup is near-instant.

**Security context**

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 100
  runAsGroup: 101
  readOnlyRootFilesystem: true       # nothing is written at runtime
  allowPrivilegeEscalation: false
  capabilities: { drop: ["ALL"] }
```

**Resources** — requests sized from measured idle usage in the test stack (see the chart's `values.yaml` for limits):

```yaml
resources:
  requests: { cpu: 100m, memory: 48Mi }
  limits:   { cpu: "500m", memory: 192Mi }
```

**Config / secrets:** no secrets. Service addresses, observability endpoints and branding vars belong in values → env.

**Telemetry:** pushed, not scraped — OTLP/gRPC to Alloy (`global.monitoring.otlpEndpoint`), profiles to Alloy `:4040`. Metrics carry `job="frontend"` and `instance=<pod>`; no scrape annotations or `ServiceMonitor` are needed.

**Helm chart:** [`platform-demo-helm/frontend`](https://github.com/platform-boutique/platform-demo-helm/tree/main/frontend) (built on the shared `platform-common` library chart). It ships with HA defaults — 2 replicas,
HPA, PodDisruptionBudget, topology spread, NetworkPolicy, restricted pod security context, probes and the full env contract (see Configuration) —
and was verified under the `restricted` Pod Security Standard on the earlier local 3-node kind cluster (since retired). The platform now targets AWS EKS, where this chart has not been deployed or verified yet.

**Deployment and admission policies:** Argo CD deploys the chart from `platform-demo-helm` (`envs/<env>/values/frontend.yaml` pins the image digest; promotion is a pull request). In the cluster Kyverno enforces, for this service: pods named `app.kubernetes.io/name: frontend` that run under the ServiceAccount of the same name (Cilium derives the network identity from that label), an image from this repository pinned by digest and signed by our release pipeline, a read-only root filesystem, resource requests and a memory limit, readiness and liveness probes and no API token. The Helm CI evaluates the rendered chart against these policies before merge; see [`platform-demo-helm/policies`](https://github.com/platform-boutique/platform-demo-helm/tree/main/policies).

## Observability

| Signal | Implementation | Destination |
|---|---|---|
| **Traces** | OpenTelemetry: `otelhttp` on the HTTP handler (inbound), `otelgrpc` client handler on every backend connection (outbound), OTLP/gRPC exporter, `AlwaysSample`, W3C TraceContext + Baggage. The trace starts here and propagates through every downstream service | Alloy `:4317` → Tempo |
| **Metrics** | `prometheus/client_golang` registry — `http_requests_total`, `http_request_duration_seconds` (buckets 1 ms … 10 s), `http_requests_in_flight`, labelled `method`, `route`, `status_code`, plus Go runtime/process collectors — bridged into OpenTelemetry and pushed over OTLP/gRPC every 15 s. `/metrics` is still served on `METRICS_PORT` for debugging | Alloy → Mimir |
| **Logs** | `logrus` JSON to stdout at `debug` level **and** an OTLP/gRPC hook (`otellogrus`): request path/method/id/session, response status/bytes/duration | Alloy → Loki |
| **Profiles** | Pyroscope Go SDK (CPU, alloc/inuse objects & space) when enabled | Alloy `:4040` → Pyroscope |

Every series carries `job="frontend"` (from `service.name`) and `instance=<pod name>` (`service.instance.id`), which is what the Grafana dashboards select on. Logs land in Loki with `service_name="frontend"` and trace/span ids, so a log line links to its trace.

## Running locally

Requirements: Go 1.25+, Docker, and the seven backend services reachable (the quickest way is `platform-demo-observability/docker-compose.yaml`).

```bash
# from source (note: entrypoint is ./cmd/server)
export PRODUCT_CATALOG_SERVICE_ADDR=localhost:3550 CURRENCY_SERVICE_ADDR=localhost:7000 \
       CART_SERVICE_ADDR=localhost:7070 RECOMMENDATION_SERVICE_ADDR=localhost:8081 \
       CHECKOUT_SERVICE_ADDR=localhost:5050 SHIPPING_SERVICE_ADDR=localhost:50051 \
       AD_SERVICE_ADDR=localhost:9555
go run ./cmd/server          # http://localhost:8080

# via Docker
docker build -t frontend .
docker run --rm -p 8080:8080 -p 9464:9464 \
  -e PRODUCT_CATALOG_SERVICE_ADDR=product-catalog-service:3550 \
  -e CURRENCY_SERVICE_ADDR=currency-service:7000 \
  -e CART_SERVICE_ADDR=cart-service:7070 \
  -e RECOMMENDATION_SERVICE_ADDR=recommendation-service:8080 \
  -e CHECKOUT_SERVICE_ADDR=checkout-service:5050 \
  -e SHIPPING_SERVICE_ADDR=shipping-service:50051 \
  -e AD_SERVICE_ADDR=ad-service:9555 \
  frontend

go test ./...

curl -s localhost:8080/_healthz            # ok
curl -s localhost:9464/metrics | grep http_requests_total
```

Browsing to `http://localhost:8080` works with the `Secure` cookies because browsers treat `localhost` as a secure context.

## Project structure

```
├── cmd/server/
│   ├── main.go          # entrypoint: env wiring, gRPC dials, router, middleware chain, HTTP server
│   ├── handlers.go      # page handlers (home, product, cart, checkout, currency, logout, product-meta)
│   ├── rpc.go           # gRPC client helpers for each backend
│   ├── middleware.go    # request logging + session-id cookie
│   ├── otlp.go          # OTLP/gRPC setup: trace, metric (Prometheus bridge) and log (logrus hook) providers
│   ├── telemetry.go     # Prometheus HTTP metrics, /metrics server, route wrapping, Pyroscope
│   └── *_test.go        # unit tests for handlers, middleware, rpc helpers and telemetry
├── internal/validator/  # form payload validation (go-playground/validator) + tests
├── money/               # Money arithmetic helpers + tests
├── genproto/            # generated gRPC stubs for all backend services (demo.pb.go, demo_grpc.pb.go)
├── templates/           # home, product, cart, order, error, header/footer, ad, recommendations, assistant
├── static/              # styles, icons, images, product photos
├── tests/e2e/           # HTTP end-to-end tests (not run by CI; the kind-based job was removed)
├── .github/workflows/   # ci.yaml (pull requests), release.yaml (v* tags / manual) - callers of platform-demo-workflows
├── Dockerfile
└── go.mod / go.sum / .golangci.yaml
```

## Known limitations

- `Secure` + `SameSite=Strict` cookies break carts over plain HTTP outside `localhost`.
- Health is static; readiness doesn't reflect backend availability.
- Debug-level JSON logging is unconditional.
- `templates/assistant.html` and the header's assistant link exist, but nothing sets the `assistant_enabled` flag and no assistant routes are registered — the AI shopping assistant is dormant.
- Per-request gRPC calls have no retries; the ad call has a 100 ms deadline, the rest use the request context.
