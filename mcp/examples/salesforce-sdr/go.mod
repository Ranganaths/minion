module github.com/yourusername/minion/mcp/examples/salesforce-sdr

go 1.21

require (
	github.com/yourusername/minion v0.1.0
)

// Use local minion for development
replace github.com/yourusername/minion => ../../../
