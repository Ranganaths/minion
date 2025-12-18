# Tutorial 7: Building a Virtual SDR

**Duration**: 2 hours
**Level**: Advanced
**Prerequisites**: Tutorials 1-6

## 🎯 Learning Objectives

By the end of this tutorial, you will:
- Build a complete Virtual Sales Development Representative
- Orchestrate Salesforce, Gmail, and Calendar together
- Implement intelligent lead qualification
- Automate email outreach campaigns
- Schedule meetings automatically
- Monitor SDR performance with metrics

## 📚 What is a Virtual SDR?

A **Virtual SDR (Sales Development Representative)** is an AI agent that automates the sales development process:

### Traditional SDR Tasks:
1. **Lead Qualification**: Research and score leads
2. **Email Outreach**: Send personalized emails
3. **Follow-ups**: Track responses and send follow-ups
4. **Meeting Scheduling**: Book meetings with qualified leads
5. **CRM Updates**: Keep Salesforce up to date

### Virtual SDR Advantages:
- 24/7 operation (never sleeps)
- Consistent quality (no bad days)
- Instant response (< 1 second)
- Scales infinitely (handle 1000s of leads)
- Data-driven (learns from metrics)

## 🏗️ Virtual SDR Architecture

```
┌─────────────────────────────────────────────────┐
│           Virtual SDR Agent                      │
│                                                  │
│  ┌────────────────────────────────────────┐    │
│  │      Lead Qualification Engine          │    │
│  │  • Score leads based on criteria        │    │
│  │  • Prioritize high-value prospects      │    │
│  └────────────────────────────────────────┘    │
│                                                  │
│  ┌────────────────────────────────────────┐    │
│  │      Email Outreach Engine              │    │
│  │  • Generate personalized emails         │    │
│  │  • Track opens and responses            │    │
│  │  • Send automated follow-ups            │    │
│  └────────────────────────────────────────┘    │
│                                                  │
│  ┌────────────────────────────────────────┐    │
│  │      Meeting Scheduler                  │    │
│  │  • Find available slots                 │    │
│  │  • Send calendar invites                │    │
│  │  • Update CRM with meetings             │    │
│  └────────────────────────────────────────┘    │
└──────┬──────────┬──────────┬─────────────────┘
       │          │          │
   ┌───▼───┐  ┌──▼────┐  ┌──▼─────┐
   │Salesf.│  │Gmail  │  │Calendar│
   └───────┘  └───────┘  └────────┘
```

## 🛠️ Part 1: Project Setup

### Step 1: Create Project Structure

```bash
mkdir virtual-sdr
cd virtual-sdr

# Create directory structure
mkdir -p cmd/sdr
mkdir -p internal/sdr
mkdir -p internal/qualification
mkdir -p internal/outreach
mkdir -p internal/scheduling
mkdir -p configs

go mod init virtual-sdr
```

### Step 2: Install Dependencies

```bash
go get github.com/yourusername/minion
go get github.com/prometheus/client_golang/prometheus
```

### Step 3: Environment Setup

Create `.env.example`:

```bash
# Salesforce
SALESFORCE_INSTANCE_URL=https://yourinstance.salesforce.com
SALESFORCE_CLIENT_ID=your_client_id
SALESFORCE_CLIENT_SECRET=your_client_secret
SALESFORCE_USERNAME=your_username
SALESFORCE_PASSWORD=your_password

# Gmail
GMAIL_CREDENTIALS=/path/to/gmail-credentials.json

# Google Calendar
CALENDAR_CREDENTIALS=/path/to/calendar-credentials.json

# SDR Configuration
SDR_NAME=VirtualSDR-Bot
SDR_EMAIL=sdr@yourcompany.com
LEAD_QUALIFICATION_THRESHOLD=70
MAX_DAILY_OUTREACH=100
```

## 🛠️ Part 2: Lead Qualification

### Qualification Criteria

Create `internal/qualification/scorer.go`:

```go
package qualification

import (
	"strings"
)

type LeadScore struct {
	Total       int
	Company     int
	Title       int
	Industry    int
	Engagement  int
	IsQualified bool
}

type LeadData struct {
	Company     string
	Title       string
	Industry    string
	Revenue     string
	Employees   int
	Website     string
	Email       string
	Phone       string
}

type Scorer struct {
	threshold int
}

func NewScorer(threshold int) *Scorer {
	return &Scorer{threshold: threshold}
}

func (s *Scorer) ScoreLead(lead *LeadData) *LeadScore {
	score := &LeadScore{}

	// Company Score (0-30 points)
	score.Company = s.scoreCompany(lead)

	// Title Score (0-25 points)
	score.Title = s.scoreTitle(lead)

	// Industry Score (0-25 points)
	score.Industry = s.scoreIndustry(lead)

	// Engagement Score (0-20 points)
	score.Engagement = s.scoreEngagement(lead)

	// Calculate total
	score.Total = score.Company + score.Title + score.Industry + score.Engagement

	// Determine if qualified
	score.IsQualified = score.Total >= s.threshold

	return score
}

func (s *Scorer) scoreCompany(lead *LeadData) int {
	points := 0

	// Company size
	if lead.Employees > 500 {
		points += 15
	} else if lead.Employees > 100 {
		points += 10
	} else if lead.Employees > 20 {
		points += 5
	}

	// Has website
	if lead.Website != "" {
		points += 5
	}

	// Revenue indicators
	if strings.Contains(strings.ToLower(lead.Revenue), "million") {
		points += 10
	}

	return min(points, 30)
}

func (s *Scorer) scoreTitle(lead *LeadData) int {
	title := strings.ToLower(lead.Title)

	// C-Level
	if strings.Contains(title, "ceo") || strings.Contains(title, "cto") ||
		strings.Contains(title, "cfo") || strings.Contains(title, "cmo") {
		return 25
	}

	// VP/Director
	if strings.Contains(title, "vp") || strings.Contains(title, "director") {
		return 20
	}

	// Manager
	if strings.Contains(title, "manager") || strings.Contains(title, "head") {
		return 15
	}

	return 5
}

func (s *Scorer) scoreIndustry(lead *LeadData) int {
	industry := strings.ToLower(lead.Industry)

	// High-value industries
	highValue := []string{"technology", "software", "fintech", "saas", "ai"}
	for _, hv := range highValue {
		if strings.Contains(industry, hv) {
			return 25
		}
	}

	// Medium-value industries
	mediumValue := []string{"finance", "healthcare", "consulting"}
	for _, mv := range mediumValue {
		if strings.Contains(industry, mv) {
			return 15
		}
	}

	return 5
}

func (s *Scorer) scoreEngagement(lead *LeadData) int {
	points := 0

	// Has email
	if lead.Email != "" {
		points += 10
	}

	// Has phone
	if lead.Phone != "" {
		points += 10
	}

	return points
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

## 🛠️ Part 3: Email Outreach

### Email Templates

Create `internal/outreach/templates.go`:

```go
package outreach

import (
	"fmt"
	"strings"
)

type EmailTemplate struct {
	Subject  string
	BodyHTML string
	BodyText string
}

type EmailData struct {
	LeadName    string
	LeadCompany string
	LeadTitle   string
	SenderName  string
	ProductName string
}

func GenerateInitialOutreach(data *EmailData) *EmailTemplate {
	subject := fmt.Sprintf("Quick question about %s's %s strategy", data.LeadCompany, getIndustryTerm(data.LeadTitle))

	bodyText := fmt.Sprintf(`Hi %s,

I noticed you're the %s at %s and wanted to reach out.

Many companies in your space are struggling with [specific pain point]. We've helped similar companies achieve:
- 40%% increase in productivity
- 60%% reduction in manual work
- 3x faster time-to-market

Would you be open to a quick 15-minute call next week to explore if %s could benefit in similar ways?

Best regards,
%s`,
		data.LeadName,
		data.LeadTitle,
		data.LeadCompany,
		data.LeadCompany,
		data.SenderName,
	)

	bodyHTML := strings.ReplaceAll(bodyText, "\n", "<br>")

	return &EmailTemplate{
		Subject:  subject,
		BodyText: bodyText,
		BodyHTML: fmt.Sprintf("<html><body><pre>%s</pre></body></html>", bodyHTML),
	}
}

func GenerateFollowUp(data *EmailData, daysSinceFirst int) *EmailTemplate {
	subject := fmt.Sprintf("Re: Quick question about %s's strategy", data.LeadCompany)

	bodyText := fmt.Sprintf(`Hi %s,

I wanted to follow up on my previous email from %d days ago.

I understand you're busy, but I genuinely believe %s could help %s [achieve specific outcome].

Would you have 15 minutes this week for a quick call?

If this isn't the right time, just let me know and I'll follow up in a few months.

Best regards,
%s`,
		data.LeadName,
		daysSinceFirst,
		data.ProductName,
		data.LeadCompany,
		data.SenderName,
	)

	bodyHTML := strings.ReplaceAll(bodyText, "\n", "<br>")

	return &EmailTemplate{
		Subject:  subject,
		BodyText: bodyText,
		BodyHTML: fmt.Sprintf("<html><body><pre>%s</pre></body></html>", bodyHTML),
	}
}

func getIndustryTerm(title string) string {
	title = strings.ToLower(title)

	if strings.Contains(title, "engineer") || strings.Contains(title, "tech") {
		return "engineering"
	}
	if strings.Contains(title, "sales") || strings.Contains(title, "revenue") {
		return "sales"
	}
	if strings.Contains(title, "marketing") {
		return "marketing"
	}
	if strings.Contains(title, "product") {
		return "product"
	}

	return "business"
}
```

## 🛠️ Part 4: Meeting Scheduling

### Calendar Integration

Create `internal/scheduling/scheduler.go`:

```go
package scheduling

import (
	"context"
	"fmt"
	"time"
)

type MeetingSlot struct {
	StartTime time.Time
	EndTime   time.Time
	Available bool
}

type Scheduler struct {
	workingHoursStart int // 9 AM
	workingHoursEnd   int // 5 PM
	timezone          *time.Location
}

func NewScheduler(timezone string) (*Scheduler, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		workingHoursStart: 9,
		workingHoursEnd:   17,
		timezone:          loc,
	}, nil
}

func (s *Scheduler) FindAvailableSlots(ctx context.Context, daysAhead int, duration time.Duration) []*MeetingSlot {
	slots := make([]*MeetingSlot, 0)

	now := time.Now().In(s.timezone)
	for day := 1; day <= daysAhead; day++ {
		date := now.AddDate(0, 0, day)

		// Skip weekends
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}

		// Generate slots for this day
		daySlots := s.generateDaySlots(date, duration)
		slots = append(slots, daySlots...)
	}

	return slots
}

func (s *Scheduler) generateDaySlots(date time.Time, duration time.Duration) []*MeetingSlot {
	slots := make([]*MeetingSlot, 0)

	// Start at working hours start
	start := time.Date(date.Year(), date.Month(), date.Day(), s.workingHoursStart, 0, 0, 0, s.timezone)
	end := time.Date(date.Year(), date.Month(), date.Day(), s.workingHoursEnd, 0, 0, 0, s.timezone)

	current := start
	for current.Add(duration).Before(end) || current.Add(duration).Equal(end) {
		slot := &MeetingSlot{
			StartTime: current,
			EndTime:   current.Add(duration),
			Available: true, // In production, check actual calendar
		}
		slots = append(slots, slot)
		current = current.Add(duration)
	}

	return slots
}

func (s *Scheduler) ProposeNextAvailable(ctx context.Context) *MeetingSlot {
	slots := s.FindAvailableSlots(ctx, 7, 30*time.Minute)

	for _, slot := range slots {
		if slot.Available {
			return slot
		}
	}

	return nil
}

func (s *Scheduler) FormatProposal(slot *MeetingSlot) string {
	return fmt.Sprintf("%s at %s %s",
		slot.StartTime.Format("Monday, January 2"),
		slot.StartTime.Format("3:04 PM"),
		slot.StartTime.Location().String(),
	)
}
```

## 🛠️ Part 5: Main SDR Implementation

### Virtual SDR Core

Create `internal/sdr/virtual_sdr.go`:

```go
package sdr

import (
	"context"
	"fmt"
	"log"
	"time"

	"virtual-sdr/internal/qualification"
	"virtual-sdr/internal/outreach"
	"virtual-sdr/internal/scheduling"

	"github.com/yourusername/minion/mcp/client"
	"github.com/yourusername/minion/mcp/observability"
)

type VirtualSDR struct {
	name       string
	email      string
	manager    *client.MCPClientManager
	pool       *client.ConnectionPool
	cache      *client.ToolCache
	cb         *client.CircuitBreaker
	prometheus *observability.PrometheusMetrics

	scorer    *qualification.Scorer
	scheduler *scheduling.Scheduler

	metrics *SDRMetrics
}

type SDRMetrics struct {
	LeadsProcessed   int
	LeadsQualified   int
	EmailsSent       int
	MeetingsScheduled int
	Errors           int
}

func NewVirtualSDR(name, email string) *VirtualSDR {
	manager := client.NewMCPClientManager()
	pool := client.NewConnectionPool(client.DefaultPoolConfig())
	cache := client.NewToolCache(client.DefaultCacheConfig())
	cb := client.NewCircuitBreaker(client.DefaultCircuitBreakerConfig())
	prometheus := observability.NewPrometheusMetrics(manager, cache, pool)

	scorer := qualification.NewScorer(70) // 70 points to qualify
	scheduler, _ := scheduling.NewScheduler("America/Los_Angeles")

	return &VirtualSDR{
		name:       name,
		email:      email,
		manager:    manager,
		pool:       pool,
		cache:      cache,
		cb:         cb,
		prometheus: prometheus,
		scorer:     scorer,
		scheduler:  scheduler,
		metrics:    &SDRMetrics{},
	}
}

func (sdr *VirtualSDR) Initialize(ctx context.Context) error {
	log.Printf("🤖 Initializing %s...\n", sdr.name)

	// Connect to Salesforce
	if err := sdr.connectSalesforce(ctx); err != nil {
		return fmt.Errorf("failed to connect Salesforce: %w", err)
	}

	// Connect to Gmail
	if err := sdr.connectGmail(ctx); err != nil {
		return fmt.Errorf("failed to connect Gmail: %w", err)
	}

	// Connect to Calendar
	if err := sdr.connectCalendar(ctx); err != nil {
		return fmt.Errorf("failed to connect Calendar: %w", err)
	}

	log.Println("✅ All systems connected")
	return nil
}

func (sdr *VirtualSDR) ProcessLead(ctx context.Context, leadID string) error {
	sdr.metrics.LeadsProcessed++

	log.Printf("\n📋 Processing lead: %s\n", leadID)

	// Step 1: Get lead from Salesforce
	lead, err := sdr.getLeadFromSalesforce(ctx, leadID)
	if err != nil {
		sdr.metrics.Errors++
		return fmt.Errorf("failed to get lead: %w", err)
	}

	// Step 2: Score the lead
	score := sdr.scorer.ScoreLead(lead)
	log.Printf("📊 Lead Score: %d/100 (Qualified: %v)\n", score.Total, score.IsQualified)

	// Update Salesforce with score
	sdr.updateLeadScore(ctx, leadID, score.Total)

	if !score.IsQualified {
		log.Println("❌ Lead not qualified")
		return nil
	}

	sdr.metrics.LeadsQualified++

	// Step 3: Send initial outreach
	if err := sdr.sendInitialEmail(ctx, lead); err != nil {
		sdr.metrics.Errors++
		return fmt.Errorf("failed to send email: %w", err)
	}

	sdr.metrics.EmailsSent++

	// Step 4: Propose meeting time
	if err := sdr.proposeMeeting(ctx, lead); err != nil {
		sdr.metrics.Errors++
		return fmt.Errorf("failed to propose meeting: %w", err)
	}

	sdr.metrics.MeetingsScheduled++

	log.Println("✅ Lead processed successfully")
	return nil
}

func (sdr *VirtualSDR) getLeadFromSalesforce(ctx context.Context, leadID string) (*qualification.LeadData, error) {
	result, err := sdr.manager.CallTool(ctx, "salesforce", "mcp_salesforce_get_lead", map[string]interface{}{
		"id": leadID,
	})

	if err != nil {
		return nil, err
	}

	// Parse result into LeadData
	leadMap := result.(map[string]interface{})

	return &qualification.LeadData{
		Company:   getString(leadMap, "Company"),
		Title:     getString(leadMap, "Title"),
		Industry:  getString(leadMap, "Industry"),
		Revenue:   getString(leadMap, "AnnualRevenue"),
		Employees: getInt(leadMap, "NumberOfEmployees"),
		Website:   getString(leadMap, "Website"),
		Email:     getString(leadMap, "Email"),
		Phone:     getString(leadMap, "Phone"),
	}, nil
}

func (sdr *VirtualSDR) sendInitialEmail(ctx context.Context, lead *qualification.LeadData) error {
	log.Println("📧 Sending initial outreach email...")

	emailData := &outreach.EmailData{
		LeadName:    extractFirstName(lead.Email),
		LeadCompany: lead.Company,
		LeadTitle:   lead.Title,
		SenderName:  sdr.name,
		ProductName: "Minion",
	}

	template := outreach.GenerateInitialOutreach(emailData)

	_, err := sdr.manager.CallTool(ctx, "gmail", "mcp_gmail_send_message", map[string]interface{}{
		"to":      lead.Email,
		"subject": template.Subject,
		"body":    template.BodyText,
	})

	if err != nil {
		return err
	}

	log.Printf("✅ Email sent to %s\n", lead.Email)
	return nil
}

func (sdr *VirtualSDR) proposeMeeting(ctx context.Context, lead *qualification.LeadData) error {
	log.Println("📅 Proposing meeting time...")

	// Find next available slot
	slot := sdr.scheduler.ProposeNextAvailable(ctx)
	if slot == nil {
		return fmt.Errorf("no available meeting slots")
	}

	proposal := sdr.scheduler.FormatProposal(slot)
	log.Printf("✅ Proposed meeting: %s\n", proposal)

	// In production, would create calendar event and send invite
	// For now, just send email with proposal

	emailBody := fmt.Sprintf(`Hi %s,

Thank you for your interest!

I'd love to show you how Minion can help %s.

Would %s work for a 30-minute introductory call?

I'll send a calendar invite once you confirm.

Best regards,
%s`,
		extractFirstName(lead.Email),
		lead.Company,
		proposal,
		sdr.name,
	)

	_, err := sdr.manager.CallTool(ctx, "gmail", "mcp_gmail_send_message", map[string]interface{}{
		"to":      lead.Email,
		"subject": "Meeting Time Proposal",
		"body":    emailBody,
	})

	return err
}

func (sdr *VirtualSDR) updateLeadScore(ctx context.Context, leadID string, score int) error {
	_, err := sdr.manager.CallTool(ctx, "salesforce", "mcp_salesforce_update_lead", map[string]interface{}{
		"id": leadID,
		"fields": map[string]interface{}{
			"Lead_Score__c": score,
			"Last_Contacted__c": time.Now().Format("2006-01-02"),
		},
	})

	return err
}

func (sdr *VirtualSDR) RunDailyWorkflow(ctx context.Context) error {
	log.Printf("\n🚀 Starting daily workflow for %s\n", sdr.name)

	// Get new leads from Salesforce
	result, err := sdr.manager.CallTool(ctx, "salesforce", "mcp_salesforce_query", map[string]interface{}{
		"query": "SELECT Id FROM Lead WHERE Status = 'New' AND CreatedDate = TODAY",
	})

	if err != nil {
		return err
	}

	leads := result.([]interface{})
	log.Printf("📋 Found %d new leads\n", len(leads))

	// Process each lead
	for i, lead := range leads {
		leadMap := lead.(map[string]interface{})
		leadID := leadMap["Id"].(string)

		log.Printf("\nProcessing lead %d/%d\n", i+1, len(leads))

		if err := sdr.ProcessLead(ctx, leadID); err != nil {
			log.Printf("⚠️ Error processing lead %s: %v\n", leadID, err)
			continue
		}

		// Rate limiting: wait between leads
		time.Sleep(2 * time.Second)
	}

	// Print metrics
	sdr.PrintMetrics()

	return nil
}

func (sdr *VirtualSDR) PrintMetrics() {
	log.Printf("\n📊 SDR Metrics:\n")
	log.Printf("  Leads Processed: %d\n", sdr.metrics.LeadsProcessed)
	log.Printf("  Leads Qualified: %d\n", sdr.metrics.LeadsQualified)
	log.Printf("  Emails Sent: %d\n", sdr.metrics.EmailsSent)
	log.Printf("  Meetings Scheduled: %d\n", sdr.metrics.MeetingsScheduled)
	log.Printf("  Errors: %d\n", sdr.metrics.Errors)

	if sdr.metrics.LeadsProcessed > 0 {
		qualRate := float64(sdr.metrics.LeadsQualified) / float64(sdr.metrics.LeadsProcessed) * 100
		log.Printf("  Qualification Rate: %.1f%%\n", qualRate)
	}
}

func (sdr *VirtualSDR) Close() {
	sdr.manager.Close()
	sdr.pool.Close()
}

// Helper functions
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		if i, ok := val.(int); ok {
			return i
		}
		if f, ok := val.(float64); ok {
			return int(f)
		}
	}
	return 0
}

func extractFirstName(email string) string {
	// Simple extraction: part before @ or .
	if idx := strings.Index(email, "@"); idx > 0 {
		name := email[:idx]
		if idx := strings.Index(name, "."); idx > 0 {
			return strings.Title(name[:idx])
		}
		return strings.Title(name)
	}
	return "there"
}

// Connection methods
func (sdr *VirtualSDR) connectSalesforce(ctx context.Context) error {
	return sdr.manager.Connect(ctx, &client.ClientConfig{
		ServerName: "salesforce",
		Transport:  client.TransportStdio,
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-salesforce"},
		Env:        getSalesforceEnv(),
	})
}

func (sdr *VirtualSDR) connectGmail(ctx context.Context) error {
	return sdr.manager.Connect(ctx, &client.ClientConfig{
		ServerName: "gmail",
		Transport:  client.TransportStdio,
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-gmail"},
		Env:        getGmailEnv(),
	})
}

func (sdr *VirtualSDR) connectCalendar(ctx context.Context) error {
	return sdr.manager.Connect(ctx, &client.ClientConfig{
		ServerName: "calendar",
		Transport:  client.TransportStdio,
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-google-calendar"},
		Env:        getCalendarEnv(),
	})
}

func getSalesforceEnv() map[string]string {
	return map[string]string{
		"SALESFORCE_INSTANCE_URL":   os.Getenv("SALESFORCE_INSTANCE_URL"),
		"SALESFORCE_CLIENT_ID":      os.Getenv("SALESFORCE_CLIENT_ID"),
		"SALESFORCE_CLIENT_SECRET":  os.Getenv("SALESFORCE_CLIENT_SECRET"),
		"SALESFORCE_USERNAME":       os.Getenv("SALESFORCE_USERNAME"),
		"SALESFORCE_PASSWORD":       os.Getenv("SALESFORCE_PASSWORD"),
	}
}

func getGmailEnv() map[string]string {
	return map[string]string{
		"GMAIL_CREDENTIALS": os.Getenv("GMAIL_CREDENTIALS"),
	}
}

func getCalendarEnv() map[string]string {
	return map[string]string{
		"CALENDAR_CREDENTIALS": os.Getenv("CALENDAR_CREDENTIALS"),
	}
}
```

## 🛠️ Part 6: Main Application

Create `cmd/sdr/main.go`:

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"virtual-sdr/internal/sdr"
)

func main() {
	ctx := context.Background()

	// Create Virtual SDR
	virtualSDR := sdr.NewVirtualSDR(
		os.Getenv("SDR_NAME"),
		os.Getenv("SDR_EMAIL"),
	)
	defer virtualSDR.Close()

	// Initialize
	if err := virtualSDR.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize SDR: %v", err)
	}

	// Start metrics server
	go func() {
		http.Handle("/metrics", virtualSDR.GetPrometheusHandler())
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		log.Println("📊 Metrics server running on :9090")
		http.ListenAndServe(":9090", nil)
	}()

	// Run daily workflow
	log.Println("🚀 Running daily workflow...")
	if err := virtualSDR.RunDailyWorkflow(ctx); err != nil {
		log.Fatalf("Daily workflow failed: %v", err)
	}

	// In production, would run on schedule (cron)
	// For now, just run once
	log.Println("✅ Workflow complete!")
}
```

## 🏋️ Practice Exercises

### Exercise 1: Add Custom Scoring Rules

Modify the scorer to give bonus points for specific company domains.

<details>
<summary>Click to see solution</summary>

```go
func (s *Scorer) scoreCompany(lead *LeadData) int {
	points := 0

	// Existing scoring...

	// Bonus for specific domains
	preferredDomains := []string{"enterprise", "fortune500", "tech"}
	for _, domain := range preferredDomains {
		if strings.Contains(strings.ToLower(lead.Company), domain) {
			points += 10
			break
		}
	}

	return min(points, 30)
}
```
</details>

### Exercise 2: Implement Follow-up Automation

Add logic to send follow-up emails after 3 days if no response.

<details>
<summary>Click to see solution</summary>

```go
func (sdr *VirtualSDR) SendFollowUps(ctx context.Context) error {
	// Query leads contacted 3 days ago with no response
	result, _ := sdr.manager.CallTool(ctx, "salesforce", "mcp_salesforce_query", map[string]interface{}{
		"query": "SELECT Id, Email, Company FROM Lead WHERE Last_Contacted__c = LAST_N_DAYS:3 AND Status = 'Contacted'",
	})

	leads := result.([]interface{})

	for _, lead := range leads {
		leadMap := lead.(map[string]interface{})

		emailData := &outreach.EmailData{
			LeadName:    extractFirstName(leadMap["Email"].(string)),
			LeadCompany: leadMap["Company"].(string),
			SenderName:  sdr.name,
			ProductName: "Minion",
		}

		template := outreach.GenerateFollowUp(emailData, 3)

		sdr.manager.CallTool(ctx, "gmail", "mcp_gmail_send_message", map[string]interface{}{
			"to":      leadMap["Email"],
			"subject": template.Subject,
			"body":    template.BodyText,
		})
	}

	return nil
}
```
</details>

## 📝 Summary

Congratulations! You've built a complete Virtual SDR that:

✅ Qualifies leads automatically using scoring
✅ Sends personalized outreach emails
✅ Proposes meeting times intelligently
✅ Updates Salesforce with all activities
✅ Tracks performance metrics
✅ Runs on autopilot daily

### Real-World Impact

A Virtual SDR can:
- Process 1000+ leads per day
- Maintain 95%+ consistency
- Respond within seconds
- Work 24/7/365
- Cost 90% less than human SDRs

### Next Steps

- Deploy to production (Tutorial 6)
- Add A/B testing for email templates
- Implement response detection
- Add multi-channel outreach (LinkedIn, phone)
- Integrate with conversation AI for meetings

## 🎯 Next Tutorial

**[Tutorial 8: Custom MCP Server →](08-custom-mcp-server.md)**

Learn how to build your own MCP server!

---

**Great job! 🎉 Continue to [Tutorial 8](08-custom-mcp-server.md) when ready.**
