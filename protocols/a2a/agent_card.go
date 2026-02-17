package a2a

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AgentCard provides a standardized schema for describing an agent's identity,
// capabilities, and endpoint information. Agent Cards enable decentralized
// discovery without requiring a central registry.
type AgentCard struct {
	// Name is the human-readable name of the agent
	Name string `json:"name"`

	// Description explains what the agent does
	Description string `json:"description"`

	// URL is the endpoint where the agent can be reached
	URL string `json:"url"`

	// Version of the agent
	Version string `json:"version"`

	// Protocol version supported
	ProtocolVersion string `json:"protocolVersion"`

	// Provider information about who created the agent
	Provider *AgentProvider `json:"provider,omitempty"`

	// Capabilities describes what the agent can do
	Capabilities AgentCapabilities `json:"capabilities"`

	// Skills are specific actions the agent can perform
	Skills []AgentSkill `json:"skills,omitempty"`

	// Authentication requirements
	Authentication *AgentAuthentication `json:"authentication,omitempty"`

	// DefaultInputModes specifies supported input modes
	DefaultInputModes []string `json:"defaultInputModes,omitempty"`

	// DefaultOutputModes specifies supported output modes
	DefaultOutputModes []string `json:"defaultOutputModes,omitempty"`

	// Metadata for additional custom information
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AgentProvider contains information about the agent's creator
type AgentProvider struct {
	// Organization name
	Organization string `json:"organization"`

	// URL of the organization
	URL string `json:"url,omitempty"`

	// Contact email
	Contact string `json:"contact,omitempty"`
}

// AgentCapabilities describes what an agent can do
type AgentCapabilities struct {
	// Streaming indicates if the agent supports SSE streaming
	Streaming bool `json:"streaming"`

	// PushNotifications indicates if the agent supports webhooks
	PushNotifications bool `json:"pushNotifications"`

	// StateTransitionHistory indicates if status history is maintained
	StateTransitionHistory bool `json:"stateTransitionHistory"`

	// InputContentTypes lists supported input MIME types
	InputContentTypes []string `json:"inputContentTypes,omitempty"`

	// OutputContentTypes lists supported output MIME types
	OutputContentTypes []string `json:"outputContentTypes,omitempty"`
}

// AgentSkill describes a specific capability of the agent
type AgentSkill struct {
	// ID uniquely identifies the skill
	ID string `json:"id"`

	// Name is the human-readable skill name
	Name string `json:"name"`

	// Description explains what the skill does
	Description string `json:"description"`

	// Tags for categorization
	Tags []string `json:"tags,omitempty"`

	// InputSchema defines expected input (JSON Schema)
	InputSchema map[string]any `json:"inputSchema,omitempty"`

	// OutputSchema defines expected output (JSON Schema)
	OutputSchema map[string]any `json:"outputSchema,omitempty"`

	// Examples of skill usage
	Examples []SkillExample `json:"examples,omitempty"`
}

// SkillExample provides an example of skill usage
type SkillExample struct {
	// Name of the example
	Name string `json:"name"`

	// Description of what the example demonstrates
	Description string `json:"description,omitempty"`

	// Input shows example input
	Input Message `json:"input"`

	// Output shows expected output
	Output Message `json:"output,omitempty"`
}

// AgentAuthentication describes authentication requirements
type AgentAuthentication struct {
	// Schemes supported (e.g., "Bearer", "Basic", "OAuth2")
	Schemes []string `json:"schemes"`

	// Required indicates if authentication is mandatory
	Required bool `json:"required"`

	// OAuth2 configuration if OAuth2 is supported
	OAuth2 *OAuth2Config `json:"oauth2,omitempty"`
}

// OAuth2Config contains OAuth2 authentication details
type OAuth2Config struct {
	// AuthorizationURL for OAuth2 flow
	AuthorizationURL string `json:"authorizationUrl,omitempty"`

	// TokenURL for obtaining tokens
	TokenURL string `json:"tokenUrl,omitempty"`

	// Scopes available
	Scopes []string `json:"scopes,omitempty"`
}

// AgentCardBuilder provides a fluent API for building agent cards
type AgentCardBuilder struct {
	card AgentCard
}

// NewAgentCardBuilder creates a new agent card builder
func NewAgentCardBuilder(name, description, url string) *AgentCardBuilder {
	return &AgentCardBuilder{
		card: AgentCard{
			Name:            name,
			Description:     description,
			URL:             url,
			ProtocolVersion: ProtocolVersion,
			Capabilities: AgentCapabilities{
				InputContentTypes:  []string{"text/plain", "application/json"},
				OutputContentTypes: []string{"text/plain", "application/json"},
			},
			DefaultInputModes:  []string{"text"},
			DefaultOutputModes: []string{"text"},
			Metadata:           make(map[string]any),
		},
	}
}

// WithVersion sets the agent version
func (b *AgentCardBuilder) WithVersion(version string) *AgentCardBuilder {
	b.card.Version = version
	return b
}

// WithProvider sets the provider information
func (b *AgentCardBuilder) WithProvider(org, url, contact string) *AgentCardBuilder {
	b.card.Provider = &AgentProvider{
		Organization: org,
		URL:          url,
		Contact:      contact,
	}
	return b
}

// WithStreaming enables streaming support
func (b *AgentCardBuilder) WithStreaming(enabled bool) *AgentCardBuilder {
	b.card.Capabilities.Streaming = enabled
	return b
}

// WithPushNotifications enables push notification support
func (b *AgentCardBuilder) WithPushNotifications(enabled bool) *AgentCardBuilder {
	b.card.Capabilities.PushNotifications = enabled
	return b
}

// WithStateHistory enables state transition history
func (b *AgentCardBuilder) WithStateHistory(enabled bool) *AgentCardBuilder {
	b.card.Capabilities.StateTransitionHistory = enabled
	return b
}

// WithInputContentTypes sets supported input content types
func (b *AgentCardBuilder) WithInputContentTypes(types ...string) *AgentCardBuilder {
	b.card.Capabilities.InputContentTypes = types
	return b
}

// WithOutputContentTypes sets supported output content types
func (b *AgentCardBuilder) WithOutputContentTypes(types ...string) *AgentCardBuilder {
	b.card.Capabilities.OutputContentTypes = types
	return b
}

// AddSkill adds a skill to the agent
func (b *AgentCardBuilder) AddSkill(skill AgentSkill) *AgentCardBuilder {
	b.card.Skills = append(b.card.Skills, skill)
	return b
}

// WithAuthentication sets authentication requirements
func (b *AgentCardBuilder) WithAuthentication(required bool, schemes ...string) *AgentCardBuilder {
	b.card.Authentication = &AgentAuthentication{
		Schemes:  schemes,
		Required: required,
	}
	return b
}

// WithOAuth2 configures OAuth2 authentication
func (b *AgentCardBuilder) WithOAuth2(authURL, tokenURL string, scopes []string) *AgentCardBuilder {
	if b.card.Authentication == nil {
		b.card.Authentication = &AgentAuthentication{
			Schemes: []string{"OAuth2"},
		}
	}
	b.card.Authentication.OAuth2 = &OAuth2Config{
		AuthorizationURL: authURL,
		TokenURL:         tokenURL,
		Scopes:           scopes,
	}
	return b
}

// WithMetadata adds custom metadata
func (b *AgentCardBuilder) WithMetadata(key string, value any) *AgentCardBuilder {
	b.card.Metadata[key] = value
	return b
}

// WithInputModes sets supported input modes
func (b *AgentCardBuilder) WithInputModes(modes ...string) *AgentCardBuilder {
	b.card.DefaultInputModes = modes
	return b
}

// WithOutputModes sets supported output modes
func (b *AgentCardBuilder) WithOutputModes(modes ...string) *AgentCardBuilder {
	b.card.DefaultOutputModes = modes
	return b
}

// Build returns the constructed agent card
func (b *AgentCardBuilder) Build() AgentCard {
	return b.card
}

// ToJSON serializes the agent card to JSON
func (c *AgentCard) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// FromJSON deserializes an agent card from JSON
func (c *AgentCard) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

// Validate checks if the agent card has required fields
func (c *AgentCard) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("agent card name is required")
	}
	if c.URL == "" {
		return fmt.Errorf("agent card URL is required")
	}
	if c.Description == "" {
		return fmt.Errorf("agent card description is required")
	}
	return nil
}

// AgentCardRegistry maintains a registry of known agents
type AgentCardRegistry struct {
	cards map[string]*AgentCard
}

// NewAgentCardRegistry creates a new agent card registry
func NewAgentCardRegistry() *AgentCardRegistry {
	return &AgentCardRegistry{
		cards: make(map[string]*AgentCard),
	}
}

// Register adds an agent card to the registry
func (r *AgentCardRegistry) Register(card *AgentCard) error {
	if err := card.Validate(); err != nil {
		return err
	}
	r.cards[card.URL] = card
	return nil
}

// Get retrieves an agent card by URL
func (r *AgentCardRegistry) Get(url string) (*AgentCard, bool) {
	card, ok := r.cards[url]
	return card, ok
}

// List returns all registered agent cards
func (r *AgentCardRegistry) List() []*AgentCard {
	cards := make([]*AgentCard, 0, len(r.cards))
	for _, card := range r.cards {
		cards = append(cards, card)
	}
	return cards
}

// Remove removes an agent card from the registry
func (r *AgentCardRegistry) Remove(url string) {
	delete(r.cards, url)
}

// FetchAgentCard fetches an agent card from a remote URL
func FetchAgentCard(url string) (*AgentCard, error) {
	return FetchAgentCardWithTimeout(url, 10*time.Second)
}

// FetchAgentCardWithTimeout fetches an agent card with a custom timeout
func FetchAgentCardWithTimeout(url string, timeout time.Duration) (*AgentCard, error) {
	client := &http.Client{Timeout: timeout}

	// Agent cards are typically served at /.well-known/agent.json
	agentCardURL := url
	if url[len(url)-1] != '/' {
		agentCardURL = url + "/.well-known/agent.json"
	} else {
		agentCardURL = url + ".well-known/agent.json"
	}

	resp, err := client.Get(agentCardURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch agent card: status %d", resp.StatusCode)
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("failed to decode agent card: %w", err)
	}

	if err := card.Validate(); err != nil {
		return nil, fmt.Errorf("invalid agent card: %w", err)
	}

	return &card, nil
}

// ServeAgentCard creates an HTTP handler for serving the agent card
func ServeAgentCard(card *AgentCard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		data, err := card.ToJSON()
		if err != nil {
			http.Error(w, "Failed to serialize agent card", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}
}
