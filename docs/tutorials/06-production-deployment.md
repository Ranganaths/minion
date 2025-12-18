# Tutorial 6: Production Deployment

**Duration**: 1.5 hours
**Level**: Intermediate
**Prerequisites**: Tutorials 1-5

## 🎯 Learning Objectives

By the end of this tutorial, you will:
- Containerize your Minion application with Docker
- Deploy to Kubernetes with proper configuration
- Implement health checks and monitoring
- Set up Prometheus and Grafana for observability
- Configure autoscaling and high availability
- Understand production best practices

## 📚 Production vs Development

| Aspect | Development | Production |
|--------|-------------|------------|
| **Environment** | Local machine | Kubernetes cluster |
| **Reliability** | Can crash and restart | Must be highly available |
| **Monitoring** | Logs to console | Prometheus + Grafana |
| **Scaling** | Single instance | Auto-scaling (3-10 pods) |
| **Secrets** | .env file | Kubernetes Secrets |
| **Health** | No checks | Liveness + Readiness probes |

## 🏗️ Production Architecture

```
┌─────────────────────────────────────────────────┐
│           Load Balancer (Ingress)                │
└──────────────┬──────────────────────────────────┘
               │
┌──────────────▼──────────────────────────────────┐
│         Minion Agent Pods (3-10 replicas)        │
│  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐           │
│  │ Pod │  │ Pod │  │ Pod │  │ Pod │ ...        │
│  │  1  │  │  2  │  │  3  │  │  N  │           │
│  └─────┘  └─────┘  └─────┘  └─────┘           │
└──────────────┬──────────────────────────────────┘
               │
┌──────────────▼──────────────────────────────────┐
│              Prometheus                          │
│          (Metrics Collection)                    │
└──────────────┬──────────────────────────────────┘
               │
┌──────────────▼──────────────────────────────────┐
│               Grafana                            │
│          (Metrics Visualization)                 │
└─────────────────────────────────────────────────┘
```

## 🛠️ Part 1: Containerization

### Step 1: Create Dockerfile

Create `Dockerfile` in your project root:

```dockerfile
# Multi-stage build for smaller image
FROM golang:1.21-alpine AS builder

# Install dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o minion-agent ./cmd/agent

# Final stage
FROM alpine:latest

# Install Node.js and npm (for MCP servers)
RUN apk add --no-cache ca-certificates nodejs npm

# Create non-root user
RUN addgroup -g 1000 minion && \
    adduser -D -u 1000 -G minion minion

WORKDIR /home/minion

# Copy binary from builder
COPY --from=builder /app/minion-agent .

# Change ownership
RUN chown -R minion:minion /home/minion

USER minion

# Expose metrics port
EXPOSE 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:9090/health || exit 1

CMD ["./minion-agent"]
```

### Step 2: Build Docker Image

```bash
# Build image
docker build -t minion-agent:v1.0.0 .

# Tag for registry
docker tag minion-agent:v1.0.0 your-registry/minion-agent:v1.0.0

# Push to registry
docker push your-registry/minion-agent:v1.0.0
```

### Step 3: Test Locally with Docker

```bash
# Run container
docker run -d \
  --name minion-agent \
  -p 9090:9090 \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=your_token \
  -e SLACK_BOT_TOKEN=your_token \
  minion-agent:v1.0.0

# Check logs
docker logs -f minion-agent

# Check health
curl http://localhost:9090/health

# Stop container
docker stop minion-agent
docker rm minion-agent
```

## 🛠️ Part 2: Docker Compose for Local Testing

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  minion-agent:
    build: .
    image: minion-agent:latest
    container_name: minion-agent
    ports:
      - "9090:9090"
    environment:
      - GITHUB_PERSONAL_ACCESS_TOKEN=${GITHUB_TOKEN}
      - SLACK_BOT_TOKEN=${SLACK_TOKEN}
      - GMAIL_CREDENTIALS=/secrets/gmail-credentials.json
      - NOTION_API_KEY=${NOTION_API_KEY}
      - LOG_LEVEL=info
    volumes:
      - ./secrets:/secrets:ro
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:9090/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s

  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    restart: unless-stopped

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana-data:/var/lib/grafana
      - ./grafana-dashboard.json:/etc/grafana/provisioning/dashboards/dashboard.json:ro
    restart: unless-stopped

volumes:
  prometheus-data:
  grafana-data:
```

Create `prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'minion-agent'
    static_configs:
      - targets: ['minion-agent:9090']
```

### Run with Docker Compose

```bash
# Start all services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f minion-agent

# Access services
# Minion Agent: http://localhost:9090/metrics
# Prometheus: http://localhost:9091
# Grafana: http://localhost:3000 (admin/admin)

# Stop all services
docker-compose down
```

## 🛠️ Part 3: Kubernetes Deployment

### Step 1: Create Namespace

Create `k8s/namespace.yaml`:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: minion
  labels:
    name: minion
```

### Step 2: Create Secrets

Create `k8s/secrets.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: minion-secrets
  namespace: minion
type: Opaque
stringData:
  github-token: "your-github-token-here"
  slack-token: "your-slack-token-here"
  notion-api-key: "your-notion-key-here"
  gmail-credentials: |
    {
      "type": "service_account",
      "project_id": "your-project",
      "private_key_id": "key-id",
      "private_key": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n",
      "client_email": "service@project.iam.gserviceaccount.com"
    }
```

**Apply secrets:**
```bash
kubectl apply -f k8s/secrets.yaml
```

### Step 3: Create ConfigMap

Create `k8s/configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: minion-config
  namespace: minion
data:
  LOG_LEVEL: "info"
  POOL_MAX_OPEN_CONNS: "10"
  POOL_MAX_IDLE_CONNS: "5"
  CACHE_MAX_ENTRIES: "100"
  CACHE_TTL: "10m"
  CIRCUIT_BREAKER_THRESHOLD: "5"
  CIRCUIT_BREAKER_TIMEOUT: "10s"
```

### Step 4: Create Deployment

Create `k8s/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: minion-agent
  namespace: minion
  labels:
    app: minion-agent
spec:
  replicas: 3
  selector:
    matchLabels:
      app: minion-agent
  template:
    metadata:
      labels:
        app: minion-agent
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      containers:
      - name: minion-agent
        image: your-registry/minion-agent:v1.0.0
        imagePullPolicy: Always
        ports:
        - containerPort: 9090
          name: metrics
          protocol: TCP
        env:
        - name: GITHUB_PERSONAL_ACCESS_TOKEN
          valueFrom:
            secretKeyRef:
              name: minion-secrets
              key: github-token
        - name: SLACK_BOT_TOKEN
          valueFrom:
            secretKeyRef:
              name: minion-secrets
              key: slack-token
        - name: NOTION_API_KEY
          valueFrom:
            secretKeyRef:
              name: minion-secrets
              key: notion-api-key
        - name: LOG_LEVEL
          valueFrom:
            configMapKeyRef:
              name: minion-config
              key: LOG_LEVEL
        volumeMounts:
        - name: gmail-credentials
          mountPath: /secrets
          readOnly: true
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 9090
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /ready
            port: 9090
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
      volumes:
      - name: gmail-credentials
        secret:
          secretName: minion-secrets
          items:
          - key: gmail-credentials
            path: gmail-credentials.json
```

### Step 5: Create Service

Create `k8s/service.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: minion-agent
  namespace: minion
  labels:
    app: minion-agent
spec:
  type: ClusterIP
  ports:
  - port: 9090
    targetPort: 9090
    protocol: TCP
    name: metrics
  selector:
    app: minion-agent
```

### Step 6: Create HorizontalPodAutoscaler

Create `k8s/hpa.yaml`:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: minion-agent-hpa
  namespace: minion
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: minion-agent
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 0
      policies:
      - type: Percent
        value: 100
        periodSeconds: 15
      - type: Pods
        value: 2
        periodSeconds: 60
```

### Step 7: Deploy to Kubernetes

```bash
# Create namespace
kubectl apply -f k8s/namespace.yaml

# Apply secrets and config
kubectl apply -f k8s/secrets.yaml
kubectl apply -f k8s/configmap.yaml

# Deploy application
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/hpa.yaml

# Check deployment status
kubectl get pods -n minion
kubectl get deployment -n minion
kubectl get svc -n minion
kubectl get hpa -n minion

# View logs
kubectl logs -f -n minion deployment/minion-agent

# Check pod health
kubectl describe pod -n minion <pod-name>
```

## 🛠️ Part 4: Monitoring with Prometheus

### ServiceMonitor for Prometheus Operator

Create `k8s/servicemonitor.yaml`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: minion-agent
  namespace: minion
  labels:
    app: minion-agent
spec:
  selector:
    matchLabels:
      app: minion-agent
  endpoints:
  - port: metrics
    interval: 15s
    path: /metrics
```

### Prometheus Alerts

Create `k8s/prometheusrule.yaml`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: minion-agent-alerts
  namespace: minion
spec:
  groups:
  - name: minion-agent
    interval: 30s
    rules:
    - alert: MinionAgentDown
      expr: up{job="minion-agent"} == 0
      for: 5m
      labels:
        severity: critical
      annotations:
        summary: "Minion agent is down"
        description: "Minion agent {{ $labels.instance }} has been down for more than 5 minutes"

    - alert: MinionHighErrorRate
      expr: rate(tool_execution_errors_total[5m]) > 0.1
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "High error rate detected"
        description: "Error rate is {{ $value }} errors/sec"

    - alert: MinionCircuitBreakerOpen
      expr: circuit_breaker_state == 1
      for: 2m
      labels:
        severity: warning
      annotations:
        summary: "Circuit breaker is open"
        description: "Circuit breaker for {{ $labels.server }} is open"

    - alert: MinionPoolExhausted
      expr: pool_connections_active / pool_connections_total > 0.9
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "Connection pool nearly exhausted"
        description: "Pool utilization is {{ $value | humanizePercentage }}"
```

## 🛠️ Part 5: Health Checks

### Implement Health Endpoints

Add to your agent code:

```go
package main

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Uptime    float64           `json:"uptime_seconds"`
	Checks    map[string]string `json:"checks"`
}

var (
	startTime = time.Now()
	isReady   atomic.Value
)

func init() {
	isReady.Store(false)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Uptime:    time.Since(startTime).Seconds(),
		Checks:    make(map[string]string),
	}

	// Check MCP connections
	if manager.IsConnected("github") {
		status.Checks["github"] = "connected"
	} else {
		status.Checks["github"] = "disconnected"
		status.Status = "degraded"
	}

	if manager.IsConnected("slack") {
		status.Checks["slack"] = "connected"
	} else {
		status.Checks["slack"] = "disconnected"
		status.Status = "degraded"
	}

	// Check circuit breaker
	if cb.IsOpen() {
		status.Checks["circuit_breaker"] = "open"
		status.Status = "degraded"
	} else {
		status.Checks["circuit_breaker"] = "closed"
	}

	// Set HTTP status code
	statusCode := http.StatusOK
	if status.Status != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(status)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	if !isReady.Load().(bool) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not ready"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
}

func main() {
	// ... initialization code ...

	// Register health endpoints
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/ready", readinessHandler)
	http.Handle("/metrics", prometheus.Handler())

	// Start initialization
	go func() {
		// Connect to MCP servers
		connectAllServers()

		// Mark as ready
		isReady.Store(true)
	}()

	// Start HTTP server
	log.Fatal(http.ListenAndServe(":9090", nil))
}
```

## 🛠️ Part 6: Graceful Shutdown

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// ... initialization code ...

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":9090",
		Handler: nil, // Using default mux
	}

	// Start server in goroutine
	go func() {
		log.Println("Starting server on :9090")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully...")

	// Mark as not ready
	isReady.Store(false)

	// Give Kubernetes time to remove pod from service
	time.Sleep(5 * time.Second)

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	// Close MCP connections
	manager.Close()

	// Close connection pool
	pool.Close()

	log.Println("Shutdown complete")
}
```

## 📊 Part 7: Grafana Dashboard

Import this dashboard JSON to Grafana:

```json
{
  "dashboard": {
    "title": "Minion Agent - Production",
    "panels": [
      {
        "title": "Tool Execution Rate",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
        "targets": [{
          "expr": "rate(tool_execution_total[5m])",
          "legendFormat": "{{server}}"
        }]
      },
      {
        "title": "Error Rate",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
        "targets": [{
          "expr": "rate(tool_execution_errors_total[5m])",
          "legendFormat": "{{server}}"
        }]
      },
      {
        "title": "Connection Pool Utilization",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8},
        "targets": [{
          "expr": "pool_connections_active / pool_connections_total",
          "legendFormat": "Utilization"
        }]
      },
      {
        "title": "Cache Hit Rate",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8},
        "targets": [{
          "expr": "cache_hit_rate",
          "legendFormat": "Hit Rate"
        }]
      },
      {
        "title": "Circuit Breaker State",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 16},
        "targets": [{
          "expr": "circuit_breaker_state",
          "legendFormat": "{{server}} (0=Closed, 1=Open, 2=HalfOpen)"
        }]
      },
      {
        "title": "Pod Count",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 16},
        "targets": [{
          "expr": "count(up{job=\"minion-agent\"})",
          "legendFormat": "Active Pods"
        }]
      }
    ]
  }
}
```

## 🏋️ Practice Exercises

### Exercise 1: Deploy to Kubernetes

Deploy the application to a Kubernetes cluster and verify it's running.

<details>
<summary>Click to see solution</summary>

```bash
# Apply all manifests
kubectl apply -f k8s/

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l app=minion-agent -n minion --timeout=60s

# Check status
kubectl get pods -n minion
kubectl get svc -n minion
kubectl get hpa -n minion

# Port forward to test
kubectl port-forward -n minion svc/minion-agent 9090:9090

# Test health endpoint
curl http://localhost:9090/health

# Check metrics
curl http://localhost:9090/metrics
```
</details>

### Exercise 2: Test Autoscaling

Generate load and watch the HPA scale up pods.

<details>
<summary>Click to see solution</summary>

```bash
# Create a load generator pod
kubectl run -n minion load-generator --image=busybox --restart=Never -- /bin/sh -c "while true; do wget -q -O- http://minion-agent:9090/metrics; done"

# Watch HPA
watch kubectl get hpa -n minion

# Watch pods
watch kubectl get pods -n minion

# After load increases, HPA will scale up to handle load

# Clean up
kubectl delete pod -n minion load-generator
```
</details>

### Exercise 3: Simulate Pod Failure

Delete a pod and verify Kubernetes restarts it automatically.

<details>
<summary>Click to see solution</summary>

```bash
# Get pod name
POD=$(kubectl get pods -n minion -l app=minion-agent -o jsonpath='{.items[0].metadata.name}')

# Delete pod
kubectl delete pod -n minion $POD

# Watch pods recover
watch kubectl get pods -n minion

# Kubernetes will automatically create a new pod to maintain desired replicas
```
</details>

## 📝 Summary

Congratulations! You've learned:

✅ How to containerize with Docker
✅ Multi-stage builds for smaller images
✅ Docker Compose for local testing
✅ Kubernetes deployment with proper configuration
✅ Health checks (liveness and readiness probes)
✅ Autoscaling with HPA
✅ Prometheus monitoring and Grafana dashboards
✅ Graceful shutdown handling

### Production Checklist

- [ ] Docker image built and pushed to registry
- [ ] Kubernetes secrets configured
- [ ] Health endpoints implemented
- [ ] Resource limits set
- [ ] Autoscaling configured
- [ ] Prometheus metrics exposed
- [ ] Grafana dashboards created
- [ ] Alerts configured
- [ ] Graceful shutdown implemented
- [ ] Tested in staging environment

## 🎯 Next Steps

**[Tutorial 7: Building a Virtual SDR →](07-virtual-sdr.md)**

Put it all together and build a production Virtual Sales Development Representative!

---

**Great job! 🎉 Continue to [Tutorial 7](07-virtual-sdr.md) when ready.**
