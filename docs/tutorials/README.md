# Minion Framework Tutorials

Welcome to the Minion framework learning path! These tutorials will take you from beginner to expert in building AI agents with Model Context Protocol (MCP) integration.

## ✅ Tutorial Completion Status

**All tutorials are now complete!** Total learning time: ~11 hours

| # | Tutorial | Duration | Status |
|---|----------|----------|--------|
| 1 | Framework Basics | 30 min | ✅ |
| 2 | MCP Integration Basics | 45 min | ✅ |
| 3 | Building Your First MCP Agent | 1 hour | ✅ |
| 4 | Advanced MCP Features | 1 hour | ✅ |
| 5 | Multi-Server Orchestration | 1 hour | ✅ |
| 6 | Production Deployment | 1.5 hours | ✅ |
| 7 | Building a Virtual SDR | 2 hours | ✅ |
| 8 | Custom MCP Server | 1.5 hours | ✅ |
| 9 | Advanced Patterns | 2 hours | ✅ |

**Quick Reference Materials:**
- [Cheat Sheet](CHEAT_SHEET.md) - One-page reference
- [Quick Reference](QUICK_REFERENCE.md) - Comprehensive API guide

---

## 📚 Learning Path

### 🟢 Beginner Level

#### [Tutorial 1: Framework Basics](01-framework-basics.md)
**Duration**: 30 minutes
**Prerequisites**: Go basics

Learn the fundamentals of the Minion framework:
- Understanding the framework architecture
- Creating your first agent
- Using built-in tools
- Agent capabilities and permissions

**What you'll build**: A simple task automation agent

---

#### [Tutorial 2: MCP Integration Basics](02-mcp-basics.md)
**Duration**: 45 minutes
**Prerequisites**: Tutorial 1

Get started with Model Context Protocol:
- What is MCP and why use it?
- Connecting to your first MCP server
- Discovering and using external tools
- Understanding tool naming and capabilities

**What you'll build**: An agent that connects to GitHub

---

#### [Tutorial 3: Building Your First MCP Agent](03-first-mcp-agent.md)
**Duration**: 1 hour
**Prerequisites**: Tutorial 1-2

Build a complete agent from scratch:
- Designing agent workflows
- Combining multiple MCP servers
- Error handling and logging
- Testing your agent

**What you'll build**: A customer support automation agent

---

### 🟡 Intermediate Level

#### [Tutorial 4: Advanced MCP Features](04-advanced-mcp-features.md)
**Duration**: 1 hour
**Prerequisites**: Tutorial 1-3

Master enterprise-grade features:
- Connection pooling for performance
- Tool caching strategies (LRU, LFU, FIFO, TTL)
- Circuit breakers for fault tolerance
- Monitoring with Prometheus metrics

**What you'll build**: A high-performance agent with all Phase 3 features

---

#### [Tutorial 5: Multi-Server Orchestration](05-multi-server-orchestration.md)
**Duration**: 1 hour
**Prerequisites**: Tutorial 1-4

Learn to orchestrate multiple services:
- Managing multiple MCP connections
- Sequential and parallel workflows
- Cross-service coordination
- Error handling strategies

**What you'll build**: A support ticket handler (GitHub + Slack + Notion + Gmail)

---

#### [Tutorial 6: Production Deployment](06-production-deployment.md)
**Duration**: 1.5 hours
**Prerequisites**: Tutorial 1-5

Deploy agents to production:
- Docker containerization
- Kubernetes deployment
- Health checks and monitoring
- Autoscaling with HPA
- Prometheus and Grafana setup

**What you'll build**: Production-ready Kubernetes deployment

---

### 🔴 Advanced Level

#### [Tutorial 7: Building a Virtual SDR](07-virtual-sdr.md)
**Duration**: 2 hours
**Prerequisites**: All previous tutorials

Build a complete AI Sales Development Representative:
- Salesforce CRM integration
- Email automation with Gmail
- Calendar scheduling
- Lead qualification engine
- Automated outreach workflows

**What you'll build**: Complete Virtual SDR system

---

#### [Tutorial 8: Custom MCP Server](08-custom-mcp-server.md)
**Duration**: 1.5 hours
**Prerequisites**: Tutorial 1-6

Create your own MCP server:
- MCP protocol specification
- Implementing tool endpoints
- Supporting stdio and HTTP transports
- Testing with Minion framework

**What you'll build**: Database MCP server with query tools

---

#### [Tutorial 9: Advanced Patterns](09-advanced-patterns.md)
**Duration**: 2 hours
**Prerequisites**: All previous tutorials

Master advanced architectural patterns:
- Retry strategies with exponential backoff
- Middleware pattern for cross-cutting concerns
- Tool chaining and composable workflows
- Adaptive caching
- Rate limiting across services
- Event-driven architecture
- Saga pattern for distributed transactions

**What you'll build**: Production-grade patterns library

---

## 🎯 Quick Start Guide

### Prerequisites

Before starting the tutorials, ensure you have:

```bash
# Go 1.21 or higher
go version

# Node.js 18 or higher (for MCP servers)
node --version

# Git
git --version

# Docker (optional, for Tutorial 6+)
docker --version

# kubectl (optional, for Tutorial 6)
kubectl version
```

### Clone the Repository

```bash
git clone https://github.com/yourusername/minion.git
cd minion
```

### Install Dependencies

```bash
go mod download
```

### Verify Installation

```bash
# Run tests
go test ./...

# Try example
cd mcp/examples/github
go run main.go
```

## 📖 Tutorial Structure

Each tutorial follows this structure:

### 1. **Learning Objectives**
Clear goals for what you'll learn

### 2. **Prerequisites**
What you need to know before starting

### 3. **Concepts**
Core concepts explained with diagrams

### 4. **Hands-On**
Step-by-step coding exercises

### 5. **Practice**
Exercises to reinforce learning

### 6. **Troubleshooting**
Common issues and solutions

### 7. **Next Steps**
What to learn next

## 🎓 Learning Approaches

### Track 1: Frontend Developer → AI Agent Builder
Best for: Frontend/Backend developers new to AI agents

```
Tutorial 1 → Tutorial 2 → Tutorial 3 → Tutorial 4 → Tutorial 7
```

**Focus**: Building practical agents quickly

---

### Track 2: DevOps Engineer → Production AI Systems
Best for: DevOps/SRE engineers deploying AI agents

```
Tutorial 1 → Tutorial 2 → Tutorial 4 → Tutorial 6 → Tutorial 9
```

**Focus**: Production deployment and scaling

---

### Track 3: Complete Mastery
Best for: Full learning path

```
Tutorial 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9
```

**Focus**: Everything from basics to advanced

---

### Track 4: Quick Prototype
Best for: Building a prototype quickly

```
Tutorial 1 → Tutorial 2 → Tutorial 3
```

**Focus**: Get productive fast

---

## 💡 Learning Tips

### 1. **Type the Code**
Don't copy-paste. Type the code yourself to build muscle memory.

### 2. **Experiment**
Try modifying the examples. Break things and fix them.

### 3. **Practice Exercises**
Complete all practice exercises before moving on.

### 4. **Build Projects**
Apply what you learn to real problems.

### 5. **Join Community**
Share your learning journey and help others.

## 🛠️ Tools & Resources

### Development Tools
- **VS Code**: Recommended IDE with Go extension
- **Postman**: For testing HTTP endpoints
- **K9s**: For Kubernetes management (Tutorial 6)

### Reference Documentation
- [Framework API Reference](../api/README.md)
- [MCP Specification](https://modelcontextprotocol.io)
- [Go Documentation](https://golang.org/doc/)

### Example Projects
- [GitHub Integration](../../mcp/examples/github/)
- [Multi-Server](../../mcp/examples/multi-server/)
- [Virtual SDR](../../mcp/examples/salesforce-sdr/)

## 📊 Estimated Time Commitment

| Track | Total Time | Best For |
|-------|-----------|----------|
| **Quick Prototype** | 2-3 hours | Fast prototyping |
| **Frontend Developer** | 5-6 hours | Building agents |
| **DevOps Engineer** | 6-7 hours | Production deployment |
| **Complete Mastery** | 10-12 hours | Full expertise |

## 🎯 Learning Outcomes

After completing these tutorials, you'll be able to:

✅ Build AI agents with the Minion framework
✅ Integrate external services via MCP protocol
✅ Implement enterprise-grade features (pooling, caching, circuit breakers)
✅ Deploy agents to production (Docker, Kubernetes)
✅ Monitor and scale agent systems
✅ Build custom MCP servers
✅ Design complex multi-agent systems

## 🤝 Contributing

Found an issue or have a suggestion?
- Open an issue: [GitHub Issues](https://github.com/yourusername/minion/issues)
- Submit improvements: [Pull Requests](https://github.com/yourusername/minion/pulls)
- Join discussions: [GitHub Discussions](https://github.com/yourusername/minion/discussions)

## 📜 License

These tutorials are part of the Minion framework and are licensed under MIT License.

## 🚀 Ready to Start?

Choose your learning track above or start with [Tutorial 1: Framework Basics](01-framework-basics.md)!

---

**Happy Learning! 🎉**
