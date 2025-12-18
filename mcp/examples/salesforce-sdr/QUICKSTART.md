# Virtual SDR - Quick Start Guide

Get your AI-powered Sales Development Representative running in 5 minutes!

## 🚀 Quick Start (Development)

### Prerequisites
- Go 1.21+
- Node.js 18+
- Docker (optional, for containers)
- Salesforce account with API access
- Google account with Gmail/Calendar API access

### 1. Clone and Setup

```bash
# Navigate to the example
cd mcp/examples/salesforce-sdr

# Copy environment template
cp .env.example .env

# Edit .env with your credentials
nano .env
```

### 2. Configure Credentials

Edit `.env` with your actual credentials:

```bash
# Salesforce
SALESFORCE_INSTANCE_URL=https://your-domain.salesforce.com
SALESFORCE_CLIENT_ID=your_client_id
SALESFORCE_CLIENT_SECRET=your_client_secret
SALESFORCE_USERNAME=you@company.com
SALESFORCE_PASSWORD=yourpassword+securitytoken

# Gmail (path to OAuth credentials JSON)
GMAIL_CREDENTIALS=./credentials/gmail-credentials.json

# Calendar (path to OAuth credentials JSON)
GOOGLE_CALENDAR_CREDENTIALS=./credentials/calendar-credentials.json
```

### 3. Build and Run

```bash
# Option 1: Using Make (recommended)
make build
make run

# Option 2: Direct Go commands
go build -o virtual-sdr main.go
source .env && ./virtual-sdr
```

### 4. Verify It's Working

Open a new terminal:

```bash
# Check health
curl http://localhost:9090/health
# Output: OK

# View detailed status
curl http://localhost:9090/status | jq

# View metrics
curl http://localhost:9090/metrics | grep mcp_
```

You should see:

```
✅ Virtual SDR started successfully

╔═══════════════════════════════════════════════════════════╗
║         🤖 Virtual SDR Agent Running                      ║
║                                                           ║
║  Endpoints:                                               ║
║    • Metrics:  http://localhost:9090/metrics             ║
║    • Health:   http://localhost:9090/health              ║
║    • Status:   http://localhost:9090/status              ║
╚═══════════════════════════════════════════════════════════╝
```

## 🐳 Quick Start (Docker)

### Using Docker Compose (Easiest)

```bash
# Start everything (agent + Prometheus + Grafana)
make docker-compose-up

# View logs
make docker-compose-logs

# Stop everything
make docker-compose-down
```

Access:
- **Virtual SDR**: http://localhost:9090
- **Prometheus**: http://localhost:9091
- **Grafana**: http://localhost:3000 (admin/admin)

### Using Docker Only

```bash
# Build and run
make docker-run
```

## ☸️ Quick Start (Kubernetes)

### Prerequisites
- kubectl configured
- Kubernetes cluster (minikube, k3s, or cloud)

### Deploy

```bash
# Deploy to Kubernetes
make k8s-deploy

# Check status
make k8s-status

# View logs
make k8s-logs

# Delete deployment
make k8s-delete
```

## 📊 View Metrics

### Terminal

```bash
# Live metrics
watch -n 1 'curl -s http://localhost:9090/metrics | grep mcp_'

# Status dashboard
watch -n 2 'curl -s http://localhost:9090/status | jq'
```

### Prometheus

1. Open http://localhost:9091 (if using docker-compose)
2. Try these queries:
   - `mcp_cache_hit_rate` - Cache performance
   - `rate(mcp_client_calls_total[5m])` - Calls per second
   - `mcp_pool_connections_active` - Active connections

### Grafana

1. Open http://localhost:3000 (admin/admin)
2. Add Prometheus data source: http://prometheus:9090
3. Import dashboard from `grafana-dashboard.json`

## 🔧 Troubleshooting

### "Connection refused" Error

**Problem**: Can't connect to MCP servers

**Solution**:
```bash
# Check Node.js is installed
node --version

# Check environment variables
cat .env

# Check MCP servers can be installed
npx -y @modelcontextprotocol/server-salesforce --help
```

### "Authentication failed" Error

**Problem**: Salesforce/Gmail credentials invalid

**Solution**:
1. Verify credentials in `.env`
2. For Salesforce: Ensure password includes security token
3. For Gmail: Re-download OAuth credentials
4. Check API access is enabled

### "Circuit breaker is open" Error

**Problem**: Too many failures, circuit breaker protecting system

**Solution**:
```bash
# Check which service is failing
curl http://localhost:9090/status | jq '.circuit_breaker'

# Wait 30 seconds for auto-recovery
# Or restart the agent
```

### Low Cache Hit Rate

**Problem**: Cache not being used effectively

**Solution**:
```bash
# Check cache metrics
curl http://localhost:9090/status | jq '.cache'

# Adjust cache settings in .env
CACHE_TTL=30m          # Increase TTL
CACHE_MAX_SIZE=200     # Increase size
```

## 🎯 Next Steps

### 1. Customize Lead Scoring

Edit `main.go` and modify `calculateLeadScore()`:

```go
func (sdr *VirtualSDR) calculateLeadScore(leadData interface{}) int {
    // Add your custom scoring logic
    score := 0

    // Example: Score by company size
    if companySize > 100 {
        score += 30
    }

    return score
}
```

### 2. Add Email Templates

Create personalized email templates:

```go
templates := map[string]string{
    "initial": "Hi {{name}}, I noticed you're interested in...",
    "followup": "Hi {{name}}, Following up on our conversation...",
    "demo": "Hi {{name}}, Here's your personalized demo link...",
}
```

### 3. Add More MCP Servers

Connect additional services:

```go
// Add Slack
sdr.connectSlack(ctx)

// Add LinkedIn
sdr.connectLinkedIn(ctx)

// Add custom API
sdr.connectCustomAPI(ctx)
```

### 4. Setup Monitoring

Configure alerts in `prometheus.yml`:

```yaml
groups:
  - name: sdr_alerts
    rules:
      - alert: HighErrorRate
        expr: rate(mcp_client_calls_failed[5m]) > 0.1
        annotations:
          summary: "High error rate detected"
```

### 5. Scale for Production

Adjust for production workload:

```bash
# In .env
POOL_MAX_OPEN_CONNS=50     # More connections
CACHE_TTL=60m              # Longer cache
CB_TIMEOUT=60s             # More tolerance

# In Kubernetes
kubectl scale deployment virtual-sdr --replicas=10 -n virtual-sdr
```

## 📚 Additional Resources

- **Full Documentation**: See [README.md](README.md)
- **MCP Specification**: https://modelcontextprotocol.io
- **Phase 3 Features**: See [../../PHASE3_COMPLETE.md](../../PHASE3_COMPLETE.md)
- **Minion Framework**: https://github.com/yourusername/minion

## 💡 Common Workflows

### Qualify Single Lead

```bash
# Via API (if you add HTTP endpoints)
curl -X POST http://localhost:9090/qualify \
  -H "Content-Type: application/json" \
  -d '{"leadId": "lead-12345"}'
```

### Manual Workflow Trigger

```bash
# Trigger daily workflow manually
curl -X POST http://localhost:9090/workflow/run
```

### Export Metrics

```bash
# Export to file
curl http://localhost:9090/metrics > metrics-$(date +%Y%m%d).txt

# Send to monitoring system
curl http://localhost:9090/metrics | \
  curl -X POST https://your-monitoring-system.com/api/v1/import --data-binary @-
```

## 🆘 Support

If you encounter issues:

1. Check logs: `make docker-compose-logs` or `make k8s-logs`
2. Verify configuration: `make validate-env`
3. Check metrics: `make metrics`
4. Review documentation: [README.md](README.md)
5. Open issue: https://github.com/yourusername/minion/issues

## 🎉 Success!

You now have a production-ready AI SDR agent with:
- ✅ Automatic lead qualification
- ✅ Personalized email outreach
- ✅ Meeting scheduling
- ✅ Enterprise-grade monitoring
- ✅ Fault-tolerant operations

**Happy selling! 🚀**
