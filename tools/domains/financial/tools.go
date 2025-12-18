package financial

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/yourusername/minion/models"
)

// InvoiceGeneratorTool creates invoices from order data
type InvoiceGeneratorTool struct{}

func (t *InvoiceGeneratorTool) Name() string {
	return "invoice_generator"
}

func (t *InvoiceGeneratorTool) Description() string {
	return "Generates professional invoices from order and customer data"
}

func (t *InvoiceGeneratorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	orderData, ok := input.Params["order_data"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid order data",
		}, nil
	}

	invoice := generateInvoice(orderData)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   invoice,
	}, nil
}

func (t *InvoiceGeneratorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "invoice_generation") || hasCapability(agent, "financial")
}

// FinancialRatioCalculatorTool calculates key financial ratios
type FinancialRatioCalculatorTool struct{}

func (t *FinancialRatioCalculatorTool) Name() string {
	return "financial_ratio_calculator"
}

func (t *FinancialRatioCalculatorTool) Description() string {
	return "Calculates key financial ratios including liquidity, profitability, and efficiency metrics"
}

func (t *FinancialRatioCalculatorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	financials, ok := input.Params["financials"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid financial data",
		}, nil
	}

	ratios := calculateFinancialRatios(financials)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   ratios,
	}, nil
}

func (t *FinancialRatioCalculatorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "financial_analysis") || hasCapability(agent, "financial")
}

// CashFlowAnalyzerTool projects and analyzes cash flows
type CashFlowAnalyzerTool struct{}

func (t *CashFlowAnalyzerTool) Name() string {
	return "cash_flow_analyzer"
}

func (t *CashFlowAnalyzerTool) Description() string {
	return "Analyzes and projects cash flows with scenario planning"
}

func (t *CashFlowAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	cashFlowData, ok := input.Params["cash_flow_data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid cash flow data",
		}, nil
	}

	analysis := analyzeCashFlow(cashFlowData)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *CashFlowAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "cash_flow_analysis") || hasCapability(agent, "financial")
}

// TaxCalculatorTool calculates tax implications
type TaxCalculatorTool struct{}

func (t *TaxCalculatorTool) Name() string {
	return "tax_calculator"
}

func (t *TaxCalculatorTool) Description() string {
	return "Calculates tax implications and estimates for various scenarios"
}

func (t *TaxCalculatorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	income, ok := input.Params["income"].(float64)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid income data",
		}, nil
	}

	jurisdiction := "US"
	if j, ok := input.Params["jurisdiction"].(string); ok {
		jurisdiction = j
	}

	taxCalc := calculateTax(income, jurisdiction)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   taxCalc,
	}, nil
}

func (t *TaxCalculatorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "tax_calculation") || hasCapability(agent, "financial")
}

// PricingOptimizerTool provides dynamic pricing recommendations
type PricingOptimizerTool struct{}

func (t *PricingOptimizerTool) Name() string {
	return "pricing_optimizer"
}

func (t *PricingOptimizerTool) Description() string {
	return "Optimizes pricing strategies based on costs, competition, and demand"
}

func (t *PricingOptimizerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	product, ok := input.Params["product"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid product data",
		}, nil
	}

	marketData, _ := input.Params["market_data"].(map[string]interface{})

	pricing := optimizePricing(product, marketData)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   pricing,
	}, nil
}

func (t *PricingOptimizerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "pricing_optimization") || hasCapability(agent, "financial")
}

// BudgetAnalyzerTool tracks budget vs actual spending
type BudgetAnalyzerTool struct{}

func (t *BudgetAnalyzerTool) Name() string {
	return "budget_analyzer"
}

func (t *BudgetAnalyzerTool) Description() string {
	return "Analyzes budget vs actual spending with variance analysis and forecasting"
}

func (t *BudgetAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	budgetData, ok := input.Params["budget_data"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid budget data",
		}, nil
	}

	analysis := analyzeBudget(budgetData)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *BudgetAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "budget_analysis") || hasCapability(agent, "financial")
}

// ROICalculatorTool calculates return on investment
type ROICalculatorTool struct{}

func (t *ROICalculatorTool) Name() string {
	return "roi_calculator"
}

func (t *ROICalculatorTool) Description() string {
	return "Calculates ROI, payback period, and IRR for investments"
}

func (t *ROICalculatorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	investment, ok := input.Params["investment"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid investment data",
		}, nil
	}

	roi := calculateROI(investment)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   roi,
	}, nil
}

func (t *ROICalculatorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "roi_calculation") || hasCapability(agent, "financial")
}

// ExpenseCategorizerTool categorizes and analyzes expenses
type ExpenseCategorizerTool struct{}

func (t *ExpenseCategorizerTool) Name() string {
	return "expense_categorizer"
}

func (t *ExpenseCategorizerTool) Description() string {
	return "Automatically categorizes expenses and provides spending insights"
}

func (t *ExpenseCategorizerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	expenses, ok := input.Params["expenses"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid expense data",
		}, nil
	}

	categorized := categorizeExpenses(expenses)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   categorized,
	}, nil
}

func (t *ExpenseCategorizerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "expense_categorization") || hasCapability(agent, "financial")
}

// BreakEvenAnalyzerTool calculates break-even points
type BreakEvenAnalyzerTool struct{}

func (t *BreakEvenAnalyzerTool) Name() string {
	return "breakeven_analyzer"
}

func (t *BreakEvenAnalyzerTool) Description() string {
	return "Calculates break-even points and conducts cost-volume-profit analysis"
}

func (t *BreakEvenAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	costData, ok := input.Params["cost_data"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid cost data",
		}, nil
	}

	analysis := analyzeBreakEven(costData)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *BreakEvenAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "breakeven_analysis") || hasCapability(agent, "financial")
}

// ProfitabilityAnalyzerTool analyzes profitability by product/segment
type ProfitabilityAnalyzerTool struct{}

func (t *ProfitabilityAnalyzerTool) Name() string {
	return "profitability_analyzer"
}

func (t *ProfitabilityAnalyzerTool) Description() string {
	return "Analyzes profitability by product, customer segment, or business unit"
}

func (t *ProfitabilityAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	data, ok := input.Params["data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid profitability data",
		}, nil
	}

	analysis := analyzeProfitability(data)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *ProfitabilityAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "profitability_analysis") || hasCapability(agent, "financial")
}

// FinancialForecastingTool generates financial forecasts
type FinancialForecastingTool struct{}

func (t *FinancialForecastingTool) Name() string {
	return "financial_forecaster"
}

func (t *FinancialForecastingTool) Description() string {
	return "Generates financial forecasts with scenario planning and sensitivity analysis"
}

func (t *FinancialForecastingTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	historicalData, ok := input.Params["historical_data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid historical data",
		}, nil
	}

	periods := 12
	if p, ok := input.Params["periods"].(int); ok {
		periods = p
	}

	forecast := generateFinancialForecast(historicalData, periods)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   forecast,
	}, nil
}

func (t *FinancialForecastingTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "financial_forecasting") || hasCapability(agent, "financial")
}

// PaymentTermsOptimizerTool optimizes payment terms
type PaymentTermsOptimizerTool struct{}

func (t *PaymentTermsOptimizerTool) Name() string {
	return "payment_terms_optimizer"
}

func (t *PaymentTermsOptimizerTool) Description() string {
	return "Optimizes payment terms to improve cash flow while maintaining customer relationships"
}

func (t *PaymentTermsOptimizerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	accountsData, ok := input.Params["accounts_data"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid accounts data",
		}, nil
	}

	optimization := optimizePaymentTerms(accountsData)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   optimization,
	}, nil
}

func (t *PaymentTermsOptimizerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "payment_optimization") || hasCapability(agent, "financial")
}

// Helper functions

func hasCapability(agent *models.Agent, capability string) bool {
	for _, cap := range agent.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

func generateInvoice(orderData map[string]interface{}) map[string]interface{} {
	invoiceNumber := fmt.Sprintf("INV-%d", time.Now().Unix())

	items, _ := orderData["items"].([]map[string]interface{})

	subtotal := 0.0
	for _, item := range items {
		if price, ok := item["price"].(float64); ok {
			if quantity, ok := item["quantity"].(float64); ok {
				subtotal += price * quantity
			}
		}
	}

	taxRate := 0.0
	if tr, ok := orderData["tax_rate"].(float64); ok {
		taxRate = tr
	} else {
		taxRate = 0.08 // Default 8%
	}

	tax := subtotal * taxRate
	total := subtotal + tax

	return map[string]interface{}{
		"invoice_number": invoiceNumber,
		"invoice_date":   time.Now().Format("2006-01-02"),
		"due_date":       time.Now().AddDate(0, 0, 30).Format("2006-01-02"),
		"customer":       orderData["customer"],
		"items":          items,
		"subtotal":       subtotal,
		"tax_rate":       taxRate * 100,
		"tax":            tax,
		"total":          total,
		"currency":       "USD",
		"payment_terms":  "Net 30",
		"status":         "pending",
	}
}

func calculateFinancialRatios(financials map[string]interface{}) map[string]interface{} {
	// Extract financial data
	currentAssets, _ := financials["current_assets"].(float64)
	currentLiabilities, _ := financials["current_liabilities"].(float64)
	totalAssets, _ := financials["total_assets"].(float64)
	totalLiabilities, _ := financials["total_liabilities"].(float64)
	revenue, _ := financials["revenue"].(float64)
	netIncome, _ := financials["net_income"].(float64)
	equity, _ := financials["equity"].(float64)
	inventory, _ := financials["inventory"].(float64)
	cogs, _ := financials["cogs"].(float64)

	ratios := map[string]interface{}{}

	// Liquidity ratios
	if currentLiabilities > 0 {
		ratios["current_ratio"] = currentAssets / currentLiabilities
		ratios["quick_ratio"] = (currentAssets - inventory) / currentLiabilities
	}

	// Profitability ratios
	if revenue > 0 {
		ratios["profit_margin"] = (netIncome / revenue) * 100
		ratios["gross_margin"] = ((revenue - cogs) / revenue) * 100
	}

	if totalAssets > 0 {
		ratios["roa"] = (netIncome / totalAssets) * 100
	}

	if equity > 0 {
		ratios["roe"] = (netIncome / equity) * 100
	}

	// Leverage ratios
	if totalAssets > 0 {
		ratios["debt_to_assets"] = (totalLiabilities / totalAssets) * 100
	}

	if equity > 0 {
		ratios["debt_to_equity"] = (totalLiabilities / equity) * 100
	}

	// Efficiency ratios
	if inventory > 0 && cogs > 0 {
		ratios["inventory_turnover"] = cogs / inventory
		ratios["days_inventory"] = 365 / (cogs / inventory)
	}

	// Interpretations
	ratios["interpretations"] = interpretRatios(ratios)

	return map[string]interface{}{
		"liquidity_ratios":    extractLiquidityRatios(ratios),
		"profitability_ratios": extractProfitabilityRatios(ratios),
		"leverage_ratios":     extractLeverageRatios(ratios),
		"efficiency_ratios":   extractEfficiencyRatios(ratios),
		"overall_health":      assessFinancialHealth(ratios),
		"recommendations":     generateFinancialRecommendations(ratios),
	}
}

func extractLiquidityRatios(ratios map[string]interface{}) map[string]float64 {
	result := make(map[string]float64)
	if val, ok := ratios["current_ratio"].(float64); ok {
		result["current_ratio"] = val
	}
	if val, ok := ratios["quick_ratio"].(float64); ok {
		result["quick_ratio"] = val
	}
	return result
}

func extractProfitabilityRatios(ratios map[string]interface{}) map[string]float64 {
	result := make(map[string]float64)
	if val, ok := ratios["profit_margin"].(float64); ok {
		result["profit_margin"] = val
	}
	if val, ok := ratios["roa"].(float64); ok {
		result["roa"] = val
	}
	if val, ok := ratios["roe"].(float64); ok {
		result["roe"] = val
	}
	return result
}

func extractLeverageRatios(ratios map[string]interface{}) map[string]float64 {
	result := make(map[string]float64)
	if val, ok := ratios["debt_to_assets"].(float64); ok {
		result["debt_to_assets"] = val
	}
	if val, ok := ratios["debt_to_equity"].(float64); ok {
		result["debt_to_equity"] = val
	}
	return result
}

func extractEfficiencyRatios(ratios map[string]interface{}) map[string]float64 {
	result := make(map[string]float64)
	if val, ok := ratios["inventory_turnover"].(float64); ok {
		result["inventory_turnover"] = val
	}
	if val, ok := ratios["days_inventory"].(float64); ok {
		result["days_inventory"] = val
	}
	return result
}

func interpretRatios(ratios map[string]interface{}) []string {
	interpretations := []string{}

	if cr, ok := ratios["current_ratio"].(float64); ok {
		if cr > 2 {
			interpretations = append(interpretations, "Strong liquidity position")
		} else if cr < 1 {
			interpretations = append(interpretations, "Potential liquidity concerns")
		}
	}

	if pm, ok := ratios["profit_margin"].(float64); ok {
		if pm > 20 {
			interpretations = append(interpretations, "Excellent profitability")
		} else if pm < 5 {
			interpretations = append(interpretations, "Low profitability margins")
		}
	}

	return interpretations
}

func assessFinancialHealth(ratios map[string]interface{}) string {
	score := 0

	if cr, ok := ratios["current_ratio"].(float64); ok && cr >= 1.5 {
		score++
	}

	if pm, ok := ratios["profit_margin"].(float64); ok && pm >= 10 {
		score++
	}

	if dte, ok := ratios["debt_to_equity"].(float64); ok && dte <= 100 {
		score++
	}

	if score >= 3 {
		return "excellent"
	} else if score >= 2 {
		return "good"
	} else if score >= 1 {
		return "fair"
	}
	return "needs_improvement"
}

func generateFinancialRecommendations(ratios map[string]interface{}) []string {
	recommendations := []string{}

	if cr, ok := ratios["current_ratio"].(float64); ok && cr < 1.5 {
		recommendations = append(recommendations, "Improve working capital management")
	}

	if pm, ok := ratios["profit_margin"].(float64); ok && pm < 10 {
		recommendations = append(recommendations, "Focus on cost reduction and revenue optimization")
	}

	if dte, ok := ratios["debt_to_equity"].(float64); ok && dte > 100 {
		recommendations = append(recommendations, "Consider reducing debt levels")
	}

	return recommendations
}

func analyzeCashFlow(cashFlowData []map[string]interface{}) map[string]interface{} {
	totalInflows := 0.0
	totalOutflows := 0.0
	netCashFlow := 0.0

	monthlyFlows := []map[string]interface{}{}

	for _, period := range cashFlowData {
		inflow, _ := period["inflow"].(float64)
		outflow, _ := period["outflow"].(float64)

		totalInflows += inflow
		totalOutflows += outflow
		net := inflow - outflow
		netCashFlow += net

		monthlyFlows = append(monthlyFlows, map[string]interface{}{
			"period":  period["period"],
			"inflow":  inflow,
			"outflow": outflow,
			"net":     net,
		})
	}

	// Calculate runway
	avgMonthlyBurn := totalOutflows / float64(len(cashFlowData))
	currentCash, _ := cashFlowData[len(cashFlowData)-1]["cash_balance"].(float64)
	runway := 0.0
	if avgMonthlyBurn > 0 {
		runway = currentCash / avgMonthlyBurn
	}

	return map[string]interface{}{
		"total_inflows":       totalInflows,
		"total_outflows":      totalOutflows,
		"net_cash_flow":       netCashFlow,
		"monthly_flows":       monthlyFlows,
		"avg_monthly_burn":    avgMonthlyBurn,
		"cash_runway_months":  runway,
		"cash_flow_health":    getCashFlowHealth(netCashFlow, runway),
		"forecast":            forecastCashFlow(monthlyFlows, 3),
		"recommendations":     getCashFlowRecommendations(netCashFlow, runway),
	}
}

func getCashFlowHealth(netCashFlow, runway float64) string {
	if netCashFlow > 0 && runway > 12 {
		return "excellent"
	} else if netCashFlow > 0 && runway > 6 {
		return "good"
	} else if runway > 3 {
		return "fair"
	}
	return "critical"
}

func forecastCashFlow(historical []map[string]interface{}, periods int) []map[string]interface{} {
	forecast := []map[string]interface{}{}

	if len(historical) == 0 {
		return forecast
	}

	// Simple average-based forecast
	avgNet := 0.0
	for _, period := range historical {
		if net, ok := period["net"].(float64); ok {
			avgNet += net
		}
	}
	avgNet /= float64(len(historical))

	for i := 1; i <= periods; i++ {
		forecast = append(forecast, map[string]interface{}{
			"period":     fmt.Sprintf("Forecast +%d", i),
			"net":        avgNet,
			"confidence": "medium",
		})
	}

	return forecast
}

func getCashFlowRecommendations(netCashFlow, runway float64) []string {
	recommendations := []string{}

	if netCashFlow < 0 {
		recommendations = append(recommendations, "Reduce operating expenses")
		recommendations = append(recommendations, "Accelerate receivables collection")
	}

	if runway < 6 {
		recommendations = append(recommendations, "Urgent: Secure additional funding")
		recommendations = append(recommendations, "Implement aggressive cost controls")
	} else if runway < 12 {
		recommendations = append(recommendations, "Begin fundraising or explore financing options")
	}

	return recommendations
}

func calculateTax(income float64, jurisdiction string) map[string]interface{} {
	// Simplified US progressive tax brackets (2024)
	var tax float64
	var effectiveRate float64

	if jurisdiction == "US" {
		switch {
		case income <= 11000:
			tax = income * 0.10
		case income <= 44725:
			tax = 1100 + (income-11000)*0.12
		case income <= 95375:
			tax = 5147 + (income-44725)*0.22
		case income <= 182100:
			tax = 16290 + (income-95375)*0.24
		case income <= 231250:
			tax = 37104 + (income-182100)*0.32
		case income <= 578125:
			tax = 52832 + (income-231250)*0.35
		default:
			tax = 174238.25 + (income-578125)*0.37
		}

		effectiveRate = (tax / income) * 100
	}

	afterTax := income - tax

	return map[string]interface{}{
		"gross_income":    income,
		"tax_owed":        tax,
		"effective_rate":  effectiveRate,
		"after_tax_income": afterTax,
		"jurisdiction":    jurisdiction,
		"breakdown":       getTaxBreakdown(income),
		"deductions":      suggestDeductions(income),
	}
}

func getTaxBreakdown(income float64) []map[string]interface{} {
	breakdown := []map[string]interface{}{
		{
			"bracket":  "10%",
			"range":    "$0 - $11,000",
			"taxable":  math.Min(income, 11000),
		},
	}

	if income > 11000 {
		breakdown = append(breakdown, map[string]interface{}{
			"bracket":  "12%",
			"range":    "$11,001 - $44,725",
			"taxable":  math.Min(income-11000, 33725),
		})
	}

	return breakdown
}

func suggestDeductions(income float64) []string {
	suggestions := []string{
		"Standard deduction: $13,850 (single)",
		"401(k) contribution limit: $22,500",
		"HSA contribution limit: $3,850",
	}

	if income > 100000 {
		suggestions = append(suggestions, "Consider tax-loss harvesting", "Maximize retirement contributions")
	}

	return suggestions
}

func optimizePricing(product map[string]interface{}, marketData map[string]interface{}) map[string]interface{} {
	cost, _ := product["cost"].(float64)
	currentPrice, _ := product["current_price"].(float64)

	// Calculate minimum price (cost + 10% margin)
	minPrice := cost * 1.1

	// Get market insights
	avgMarketPrice := 0.0
	if marketData != nil {
		avgMarketPrice, _ = marketData["average_price"].(float64)
	}

	if avgMarketPrice == 0 {
		avgMarketPrice = cost * 2 // Default 100% markup
	}

	// Calculate optimal price using cost-plus and market-based pricing
	targetMargin := 0.40 // 40% margin
	costPlusPrice := cost / (1 - targetMargin)

	// Weight 60% cost-plus, 40% market
	optimalPrice := costPlusPrice*0.6 + avgMarketPrice*0.4

	// Calculate price elasticity scenario
	scenarios := []map[string]interface{}{
		{
			"name":           "aggressive",
			"price":          optimalPrice * 0.9,
			"expected_volume": "high",
			"expected_revenue": optimalPrice * 0.9 * 1.3,
		},
		{
			"name":           "recommended",
			"price":          optimalPrice,
			"expected_volume": "medium",
			"expected_revenue": optimalPrice * 1.0,
		},
		{
			"name":           "premium",
			"price":          optimalPrice * 1.15,
			"expected_volume": "low",
			"expected_revenue": optimalPrice * 1.15 * 0.8,
		},
	}

	return map[string]interface{}{
		"current_price":     currentPrice,
		"cost":              cost,
		"min_price":         minPrice,
		"optimal_price":     optimalPrice,
		"market_avg_price":  avgMarketPrice,
		"recommended_price": optimalPrice,
		"price_change_pct":  ((optimalPrice - currentPrice) / currentPrice) * 100,
		"scenarios":         scenarios,
		"rationale":         getPricingRationale(cost, optimalPrice, avgMarketPrice),
	}
}

func getPricingRationale(cost, optimalPrice, marketPrice float64) []string {
	rationale := []string{
		fmt.Sprintf("Cost-based pricing ensures %.1f%% margin", ((optimalPrice-cost)/optimalPrice)*100),
	}

	if marketPrice > 0 {
		diff := ((optimalPrice - marketPrice) / marketPrice) * 100
		if diff > 10 {
			rationale = append(rationale, "Premium positioning vs market average")
		} else if diff < -10 {
			rationale = append(rationale, "Value positioning vs market average")
		} else {
			rationale = append(rationale, "Competitive with market average")
		}
	}

	return rationale
}

func analyzeBudget(budgetData map[string]interface{}) map[string]interface{} {
	budget, _ := budgetData["budget"].(float64)
	actual, _ := budgetData["actual"].(float64)

	variance := actual - budget
	variancePct := (variance / budget) * 100

	status := "on_track"
	if variancePct > 10 {
		status = "over_budget"
	} else if variancePct < -10 {
		status = "under_budget"
	}

	// Category breakdown
	categories, _ := budgetData["categories"].([]map[string]interface{})
	categoryAnalysis := []map[string]interface{}{}

	for _, cat := range categories {
		catBudget, _ := cat["budget"].(float64)
		catActual, _ := cat["actual"].(float64)
		catVariance := catActual - catBudget

		categoryAnalysis = append(categoryAnalysis, map[string]interface{}{
			"name":         cat["name"],
			"budget":       catBudget,
			"actual":       catActual,
			"variance":     catVariance,
			"variance_pct": (catVariance / catBudget) * 100,
		})
	}

	return map[string]interface{}{
		"budget":            budget,
		"actual":            actual,
		"variance":          variance,
		"variance_pct":      variancePct,
		"status":            status,
		"category_analysis": categoryAnalysis,
		"forecast":          forecastBudget(budgetData),
		"recommendations":   getBudgetRecommendations(status, categoryAnalysis),
	}
}

func forecastBudget(budgetData map[string]interface{}) map[string]interface{} {
	actual, _ := budgetData["actual"].(float64)
	periodsElapsed, _ := budgetData["periods_elapsed"].(float64)
	totalPeriods, _ := budgetData["total_periods"].(float64)

	if periodsElapsed == 0 {
		periodsElapsed = 6 // default to mid-year
		totalPeriods = 12
	}

	runRate := actual / periodsElapsed
	forecastYear := runRate * totalPeriods

	return map[string]interface{}{
		"current_run_rate":  runRate,
		"year_end_forecast": forecastYear,
		"confidence":        "medium",
	}
}

func getBudgetRecommendations(status string, categoryAnalysis []map[string]interface{}) []string {
	recommendations := []string{}

	if status == "over_budget" {
		recommendations = append(recommendations, "Implement cost control measures")
		recommendations = append(recommendations, "Review discretionary spending")
	}

	// Find categories significantly over budget
	for _, cat := range categoryAnalysis {
		if varPct, ok := cat["variance_pct"].(float64); ok && varPct > 20 {
			name, _ := cat["name"].(string)
			recommendations = append(recommendations, fmt.Sprintf("Investigate overspending in %s category", name))
		}
	}

	return recommendations
}

func calculateROI(investment map[string]interface{}) map[string]interface{} {
	initialInvestment, _ := investment["initial_investment"].(float64)
	cashFlows, _ := investment["cash_flows"].([]float64)

	if len(cashFlows) == 0 {
		return map[string]interface{}{
			"error": "No cash flows provided",
		}
	}

	totalReturns := 0.0
	for _, cf := range cashFlows {
		totalReturns += cf
	}

	netProfit := totalReturns - initialInvestment
	roi := (netProfit / initialInvestment) * 100

	// Calculate payback period
	paybackPeriod := calculatePaybackPeriod(initialInvestment, cashFlows)

	// Calculate IRR (simplified)
	irr := calculateIRR(initialInvestment, cashFlows)

	return map[string]interface{}{
		"initial_investment": initialInvestment,
		"total_returns":      totalReturns,
		"net_profit":         netProfit,
		"roi_percentage":     roi,
		"payback_period":     paybackPeriod,
		"irr":                irr,
		"assessment":         assessROI(roi, paybackPeriod),
		"recommendations":    getROIRecommendations(roi),
	}
}

func calculatePaybackPeriod(investment float64, cashFlows []float64) float64 {
	cumulative := 0.0
	for i, cf := range cashFlows {
		cumulative += cf
		if cumulative >= investment {
			// Linear interpolation for the fractional period
			excess := cumulative - investment
			fraction := 1 - (excess / cf)
			return float64(i) + fraction
		}
	}
	return -1 // Never pays back
}

func calculateIRR(investment float64, cashFlows []float64) float64 {
	// Simplified IRR calculation using approximation
	totalReturns := 0.0
	for _, cf := range cashFlows {
		totalReturns += cf
	}

	avgReturn := totalReturns / float64(len(cashFlows))
	irr := ((avgReturn - investment) / investment) / float64(len(cashFlows)) * 100

	return irr
}

func assessROI(roi, payback float64) string {
	if roi > 50 && payback < 2 {
		return "excellent"
	} else if roi > 25 && payback < 3 {
		return "good"
	} else if roi > 10 {
		return "acceptable"
	}
	return "poor"
}

func getROIRecommendations(roi float64) []string {
	if roi < 15 {
		return []string{
			"Consider alternative investments",
			"Review cost assumptions",
			"Explore ways to increase returns",
		}
	} else if roi > 50 {
		return []string{
			"Excellent investment opportunity",
			"Consider scaling investment",
		}
	}
	return []string{
		"Solid investment with reasonable returns",
	}
}

func categorizeExpenses(expenses []map[string]interface{}) map[string]interface{} {
	categories := make(map[string]float64)
	categorized := []map[string]interface{}{}

	for _, expense := range expenses {
		description, _ := expense["description"].(string)
		amount, _ := expense["amount"].(float64)

		category := detectCategory(description)
		categories[category] += amount

		expense["category"] = category
		categorized = append(categorized, expense)
	}

	total := 0.0
	for _, amount := range categories {
		total += amount
	}

	return map[string]interface{}{
		"categorized_expenses": categorized,
		"category_totals":      categories,
		"total_expenses":       total,
		"top_categories":       getTopCategories(categories, 5),
		"insights":             generateExpenseInsights(categories, total),
	}
}

func detectCategory(description string) string {
	desc := fmt.Sprintf("%s", description)
	desc = fmt.Sprintf("%v", desc) // Convert to lowercase

	categoryKeywords := map[string][]string{
		"Travel":       {"flight", "hotel", "uber", "taxi", "travel"},
		"Office":       {"office", "supplies", "equipment"},
		"Marketing":    {"marketing", "advertising", "campaign"},
		"Software":     {"saas", "software", "subscription", "license"},
		"Utilities":    {"utilities", "internet", "phone"},
		"Payroll":      {"payroll", "salary", "wages"},
		"Professional": {"legal", "consulting", "professional"},
	}

	for category, keywords := range categoryKeywords {
		for _, keyword := range keywords {
			if contains(desc, keyword) {
				return category
			}
		}
	}

	return "Other"
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || fmt.Sprintf("%v", s) == fmt.Sprintf("%v", substr))
}

func getTopCategories(categories map[string]float64, limit int) []map[string]interface{} {
	type catAmount struct {
		category string
		amount   float64
	}

	var cats []catAmount
	for cat, amt := range categories {
		cats = append(cats, catAmount{cat, amt})
	}

	// Sort by amount descending
	for i := 0; i < len(cats)-1; i++ {
		for j := i + 1; j < len(cats); j++ {
			if cats[j].amount > cats[i].amount {
				cats[i], cats[j] = cats[j], cats[i]
			}
		}
	}

	result := []map[string]interface{}{}
	for i := 0; i < len(cats) && i < limit; i++ {
		result = append(result, map[string]interface{}{
			"category": cats[i].category,
			"amount":   cats[i].amount,
		})
	}

	return result
}

func generateExpenseInsights(categories map[string]float64, total float64) []string {
	insights := []string{}

	for category, amount := range categories {
		pct := (amount / total) * 100
		if pct > 30 {
			insights = append(insights, fmt.Sprintf("%s represents %.1f%% of total expenses", category, pct))
		}
	}

	return insights
}

func analyzeBreakEven(costData map[string]interface{}) map[string]interface{} {
	fixedCosts, _ := costData["fixed_costs"].(float64)
	variableCostPerUnit, _ := costData["variable_cost_per_unit"].(float64)
	pricePerUnit, _ := costData["price_per_unit"].(float64)

	if pricePerUnit <= variableCostPerUnit {
		return map[string]interface{}{
			"error": "Price per unit must be greater than variable cost per unit",
		}
	}

	contributionMargin := pricePerUnit - variableCostPerUnit
	breakEvenUnits := fixedCosts / contributionMargin
	breakEvenRevenue := breakEvenUnits * pricePerUnit

	// Margin of safety
	currentUnits, _ := costData["current_units"].(float64)
	marginOfSafety := 0.0
	if currentUnits > 0 {
		marginOfSafety = ((currentUnits - breakEvenUnits) / currentUnits) * 100
	}

	return map[string]interface{}{
		"break_even_units":     breakEvenUnits,
		"break_even_revenue":   breakEvenRevenue,
		"contribution_margin":  contributionMargin,
		"contribution_margin_ratio": (contributionMargin / pricePerUnit) * 100,
		"margin_of_safety_pct": marginOfSafety,
		"sensitivity_analysis": performSensitivityAnalysis(fixedCosts, variableCostPerUnit, pricePerUnit),
		"recommendations":      getBreakEvenRecommendations(marginOfSafety),
	}
}

func performSensitivityAnalysis(fixedCosts, variableCost, price float64) map[string]interface{} {
	scenarios := []map[string]interface{}{
		{
			"scenario":       "10% price increase",
			"new_breakeven":  fixedCosts / ((price * 1.1) - variableCost),
			"impact":         "positive",
		},
		{
			"scenario":       "10% cost increase",
			"new_breakeven":  fixedCosts / (price - (variableCost * 1.1)),
			"impact":         "negative",
		},
	}

	return map[string]interface{}{
		"scenarios": scenarios,
	}
}

func getBreakEvenRecommendations(marginOfSafety float64) []string {
	if marginOfSafety < 20 {
		return []string{
			"Low margin of safety - consider reducing fixed costs",
			"Explore price increases if market allows",
			"Focus on volume growth",
		}
	}
	return []string{
		"Healthy margin of safety",
		"Continue monitoring cost structure",
	}
}

func analyzeProfitability(data []map[string]interface{}) map[string]interface{} {
	profitabilityBySegment := []map[string]interface{}{}

	totalRevenue := 0.0
	totalCost := 0.0

	for _, segment := range data {
		revenue, _ := segment["revenue"].(float64)
		cost, _ := segment["cost"].(float64)

		profit := revenue - cost
		margin := 0.0
		if revenue > 0 {
			margin = (profit / revenue) * 100
		}

		totalRevenue += revenue
		totalCost += cost

		profitabilityBySegment = append(profitabilityBySegment, map[string]interface{}{
			"segment": segment["segment"],
			"revenue": revenue,
			"cost":    cost,
			"profit":  profit,
			"margin":  margin,
			"roi":     (profit / cost) * 100,
		})
	}

	totalProfit := totalRevenue - totalCost

	return map[string]interface{}{
		"total_revenue":           totalRevenue,
		"total_cost":              totalCost,
		"total_profit":            totalProfit,
		"overall_margin":          (totalProfit / totalRevenue) * 100,
		"segment_analysis":        profitabilityBySegment,
		"most_profitable":         findMostProfitable(profitabilityBySegment),
		"least_profitable":        findLeastProfitable(profitabilityBySegment),
		"recommendations":         getProfitabilityRecommendations(profitabilityBySegment),
	}
}

func findMostProfitable(segments []map[string]interface{}) map[string]interface{} {
	if len(segments) == 0 {
		return nil
	}

	most := segments[0]
	maxProfit, _ := most["profit"].(float64)

	for _, seg := range segments {
		if profit, ok := seg["profit"].(float64); ok && profit > maxProfit {
			maxProfit = profit
			most = seg
		}
	}

	return most
}

func findLeastProfitable(segments []map[string]interface{}) map[string]interface{} {
	if len(segments) == 0 {
		return nil
	}

	least := segments[0]
	minProfit, _ := least["profit"].(float64)

	for _, seg := range segments {
		if profit, ok := seg["profit"].(float64); ok && profit < minProfit {
			minProfit = profit
			least = seg
		}
	}

	return least
}

func getProfitabilityRecommendations(segments []map[string]interface{}) []string {
	recommendations := []string{}

	for _, seg := range segments {
		margin, _ := seg["margin"].(float64)
		segmentName, _ := seg["segment"].(string)

		if margin < 10 {
			recommendations = append(recommendations,
				fmt.Sprintf("Review %s segment - low margins", segmentName))
		} else if margin > 50 {
			recommendations = append(recommendations,
				fmt.Sprintf("Scale %s segment - high margins", segmentName))
		}
	}

	return recommendations
}

func generateFinancialForecast(historicalData []map[string]interface{}, periods int) map[string]interface{} {
	if len(historicalData) < 3 {
		return map[string]interface{}{
			"error": "Insufficient historical data for forecasting",
		}
	}

	revenues := extractValues(historicalData, "revenue")
	expenses := extractValues(historicalData, "expenses")

	revenueForecast := forecastMetric(revenues, periods)
	expenseForecast := forecastMetric(expenses, periods)

	scenarios := []map[string]interface{}{
		{
			"scenario":  "base",
			"revenue":   revenueForecast,
			"expenses":  expenseForecast,
			"profit":    subtractArrays(revenueForecast, expenseForecast),
		},
		{
			"scenario":  "optimistic",
			"revenue":   multiplyArray(revenueForecast, 1.15),
			"expenses":  expenseForecast,
			"profit":    subtractArrays(multiplyArray(revenueForecast, 1.15), expenseForecast),
		},
		{
			"scenario":  "pessimistic",
			"revenue":   multiplyArray(revenueForecast, 0.85),
			"expenses":  multiplyArray(expenseForecast, 1.05),
			"profit":    subtractArrays(multiplyArray(revenueForecast, 0.85), multiplyArray(expenseForecast, 1.05)),
		},
	}

	return map[string]interface{}{
		"forecast_periods": periods,
		"scenarios":        scenarios,
		"confidence":       "medium",
		"assumptions":      []string{
			"Historical trends continue",
			"No major market disruptions",
			"Consistent operational capacity",
		},
	}
}

func extractValues(data []map[string]interface{}, key string) []float64 {
	values := []float64{}
	for _, item := range data {
		if val, ok := item[key].(float64); ok {
			values = append(values, val)
		}
	}
	return values
}

func forecastMetric(historical []float64, periods int) []float64 {
	if len(historical) == 0 {
		return []float64{}
	}

	// Simple linear trend
	n := float64(len(historical))
	var sumX, sumY, sumXY, sumX2 float64

	for i, y := range historical {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	forecast := make([]float64, periods)
	lastX := float64(len(historical) - 1)

	for i := 0; i < periods; i++ {
		x := lastX + float64(i+1)
		forecast[i] = slope*x + intercept
	}

	return forecast
}

func multiplyArray(arr []float64, multiplier float64) []float64 {
	result := make([]float64, len(arr))
	for i, val := range arr {
		result[i] = val * multiplier
	}
	return result
}

func subtractArrays(arr1, arr2 []float64) []float64 {
	minLen := len(arr1)
	if len(arr2) < minLen {
		minLen = len(arr2)
	}

	result := make([]float64, minLen)
	for i := 0; i < minLen; i++ {
		result[i] = arr1[i] - arr2[i]
	}
	return result
}

func optimizePaymentTerms(accountsData map[string]interface{}) map[string]interface{} {
	currentDSO, _ := accountsData["current_dso"].(float64) // Days Sales Outstanding
	avgInvoiceValue, _ := accountsData["avg_invoice_value"].(float64)
	annualRevenue, _ := accountsData["annual_revenue"].(float64)

	// Recommendations for payment terms
	recommendations := []map[string]interface{}{
		{
			"term":             "Net 15",
			"projected_dso":    15.0,
			"cash_improvement": calculateCashImprovement(currentDSO, 15, annualRevenue),
			"risk":             "May impact customer satisfaction",
		},
		{
			"term":             "Net 30",
			"projected_dso":    30.0,
			"cash_improvement": calculateCashImprovement(currentDSO, 30, annualRevenue),
			"risk":             "Balanced approach",
		},
		{
			"term":             "2/10 Net 30",
			"projected_dso":    20.0,
			"cash_improvement": calculateCashImprovement(currentDSO, 20, annualRevenue),
			"risk":             "Incentivizes early payment",
		},
	}

	return map[string]interface{}{
		"current_dso":       currentDSO,
		"target_dso":        25.0,
		"recommendations":   recommendations,
		"estimated_impact":  estimatePaymentTermsImpact(currentDSO, annualRevenue),
	}
}

func calculateCashImprovement(currentDSO, newDSO, revenue float64) float64 {
	dailyRevenue := revenue / 365
	return (currentDSO - newDSO) * dailyRevenue
}

func estimatePaymentTermsImpact(dso, revenue float64) map[string]interface{} {
	if dso <= 30 {
		return map[string]interface{}{
			"status":  "healthy",
			"message": "Payment terms are within industry standards",
		}
	}

	dailyRevenue := revenue / 365
	excessDays := dso - 30
	tiedUpCash := excessDays * dailyRevenue

	return map[string]interface{}{
		"status":       "needs_improvement",
		"tied_up_cash": tiedUpCash,
		"message":      fmt.Sprintf("%.0f days above target ties up $%.2f", excessDays, tiedUpCash),
	}
}
