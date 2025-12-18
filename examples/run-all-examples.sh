#!/bin/bash

# Minion Framework - Run All Examples
# This script runs all integration examples in sequence

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Banner
echo -e "${BLUE}"
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║           Minion Framework - Integration Examples           ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go is not installed. Please install Go 1.24+ first.${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Go version: $(go version)${NC}\n"

# Function to run an example
run_example() {
    local dir=$1
    local name=$2
    local icon=$3

    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}${icon} Running: ${name}${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}\n"

    if [ -d "$dir" ]; then
        cd "$dir"

        if [ -f "main.go" ]; then
            echo -e "${GREEN}📂 Directory: $dir${NC}"
            echo -e "${GREEN}▶️  Executing...${NC}\n"

            if go run main.go; then
                echo -e "\n${GREEN}✅ $name completed successfully!${NC}\n"
            else
                echo -e "\n${RED}❌ $name failed!${NC}\n"
                exit 1
            fi
        else
            echo -e "${YELLOW}⚠️  main.go not found in $dir${NC}\n"
        fi

        cd - > /dev/null
    else
        echo -e "${YELLOW}⚠️  Directory $dir not found${NC}\n"
    fi

    # Pause between examples
    sleep 2
}

# Main execution
echo -e "${BLUE}🚀 Starting all integration examples...${NC}\n"

# Example 1: DevOps Automation
run_example "devops-automation" "DevOps Automation" "🚀"

# Example 2: Customer Support
run_example "customer-support" "Customer Support Automation" "🎧"

# Example 3: Sales Automation
run_example "sales-automation" "Sales Pipeline Automation" "💰"

# Summary
echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}           ✨ All Examples Completed Successfully! ✨${NC}"
echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}\n"

echo -e "${BLUE}📊 Summary:${NC}"
echo -e "  ✅ DevOps Automation - GitHub, Jira, Slack integration"
echo -e "  ✅ Customer Support - Email processing, sentiment analysis"
echo -e "  ✅ Sales Automation - Lead scoring, revenue forecasting"
echo ""
echo -e "${BLUE}📚 Next Steps:${NC}"
echo -e "  1. Review example code in each directory"
echo -e "  2. Customize workflows for your use case"
echo -e "  3. Connect to real APIs with credentials"
echo -e "  4. Build your own automation agents!"
echo ""
echo -e "${GREEN}🎉 Happy automating with Minion Framework!${NC}\n"
