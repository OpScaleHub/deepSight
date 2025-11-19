# deepSight

A tiny Go web app that provides a simple uptime and runtime dashboard. Built to run in a container and as a Kubernetes pod.

Features:
- Dashboard: `/` — uptime, requests, host, memory, and a 60s sparkline
- Health: `/health` — readiness/liveness friendly
- Metrics: `/metrics` — simple Prometheus-friendly metrics

Quick start (local build):

```bash
# build binary
go build -o deepsight .

# run locally
PORT=8080 ./deepsight
```

Build Docker image:

```bash
docker build -t ghcr.io/opscalehub/deepsight/workload:latest .
```

Push to registry and update image in `k8s/deployment.yaml`.

Deploy to Kubernetes:

```bash
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

Check pods and port-forward to view dashboard locally:

```bash
kubectl get pods -l app=deepsight
kubectl port-forward svc/deepsight 8080:80
# open http://localhost:8080
```

Notes:
- Update the image name in `k8s/deployment.yaml` to match your registry.
- The app serves static files from `/static` and templates from `templates` at runtime — the Dockerfile copies the full repo before building.

# adding CD
