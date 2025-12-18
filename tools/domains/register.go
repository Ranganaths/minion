package domains

import (
	"github.com/Ranganaths/minion/core"
	"github.com/Ranganaths/minion/tools"
	"github.com/Ranganaths/minion/tools/domains/analytics"
	"github.com/Ranganaths/minion/tools/domains/communication"
	"github.com/Ranganaths/minion/tools/domains/customer"
	"github.com/Ranganaths/minion/tools/domains/financial"
	"github.com/Ranganaths/minion/tools/domains/integration"
	"github.com/Ranganaths/minion/tools/domains/marketing"
	"github.com/Ranganaths/minion/tools/domains/projectmgmt"
	"github.com/Ranganaths/minion/tools/domains/sales"
)

// RegisterAllDomainTools registers all domain-specific tools with the framework
func RegisterAllDomainTools(framework core.Framework) error {
	// Register Sales tools
	if err := RegisterSalesTools(framework); err != nil {
		return err
	}

	// Register Marketing tools
	if err := RegisterMarketingTools(framework); err != nil {
		return err
	}

	// Register Data & Analytics tools
	if err := RegisterAnalyticsTools(framework); err != nil {
		return err
	}

	// Register Customer & Support tools
	if err := RegisterCustomerTools(framework); err != nil {
		return err
	}

	// Register Financial tools
	if err := RegisterFinancialTools(framework); err != nil {
		return err
	}

	// Register Integration & External tools
	if err := RegisterIntegrationTools(framework); err != nil {
		return err
	}

	// Register Communication & Collaboration tools
	if err := RegisterCommunicationTools(framework); err != nil {
		return err
	}

	// Register Project Management tools
	if err := RegisterProjectManagementTools(framework); err != nil {
		return err
	}

	return nil
}

// RegisterSalesTools registers all Sales Analyst tools
func RegisterSalesTools(framework core.Framework) error {
	salesTools := []tools.Tool{
		&sales.RevenueAnalyzerTool{},
		&sales.PipelineAnalyzerTool{},
		&sales.CustomerSegmentationTool{},
		&sales.DealScoringTool{},
		&sales.ForecastingTool{},
		&sales.ConversionRateAnalyzerTool{},
		&sales.ChurnPredictorTool{},
		&sales.QuotaAttainmentTool{},
	}

	for _, tool := range salesTools {
		if err := framework.RegisterTool(tool); err != nil {
			return err
		}
	}

	return nil
}

// RegisterMarketingTools registers all Marketing Analyst tools
func RegisterMarketingTools(framework core.Framework) error {
	marketingTools := []tools.Tool{
		&marketing.CampaignROICalculatorTool{},
		&marketing.FunnelAnalyzerTool{},
		&marketing.CACCalculatorTool{},
		&marketing.AttributionAnalyzerTool{},
		&marketing.ABTestAnalyzerTool{},
		&marketing.EngagementScorerTool{},
		&marketing.ContentPerformanceTool{},
		&marketing.LeadScoringTool{},
		&marketing.EmailCampaignAnalyzerTool{},
	}

	for _, tool := range marketingTools {
		if err := framework.RegisterTool(tool); err != nil {
			return err
		}
	}

	return nil
}

// RegisterAnalyticsTools registers all Data & Analytics tools
func RegisterAnalyticsTools(framework core.Framework) error {
	analyticsTools := []tools.Tool{
		&analytics.SQLGeneratorTool{},
		&analytics.AnomalyDetectorTool{},
		&analytics.CorrelationAnalyzerTool{},
		&analytics.TrendPredictorTool{},
		&analytics.DataValidatorTool{},
		&analytics.ReportGeneratorTool{},
		&analytics.DataTransformerTool{},
		&analytics.StatisticalAnalyzerTool{},
		&analytics.DataProfilingTool{},
		&analytics.TimeSeriesAnalyzerTool{},
	}

	for _, tool := range analyticsTools {
		if err := framework.RegisterTool(tool); err != nil {
			return err
		}
	}

	return nil
}

// RegisterCustomerTools registers all Customer & Support tools
func RegisterCustomerTools(framework core.Framework) error {
	customerTools := []tools.Tool{
		&customer.SentimentAnalyzerTool{},
		&customer.TicketClassifierTool{},
		&customer.ResponseGeneratorTool{},
		&customer.CustomerHealthScorerTool{},
		&customer.FeedbackAnalyzerTool{},
		&customer.NPSCalculatorTool{},
		&customer.SupportMetricsAnalyzerTool{},
		&customer.CSATAnalyzerTool{},
		&customer.TicketRoutingTool{},
		&customer.KnowledgeBaseSearchTool{},
		&customer.CustomerJourneyAnalyzerTool{},
	}

	for _, tool := range customerTools {
		if err := framework.RegisterTool(tool); err != nil {
			return err
		}
	}

	return nil
}

// RegisterFinancialTools registers all Financial tools
func RegisterFinancialTools(framework core.Framework) error {
	financialTools := []tools.Tool{
		&financial.InvoiceGeneratorTool{},
		&financial.FinancialRatioCalculatorTool{},
		&financial.CashFlowAnalyzerTool{},
		&financial.TaxCalculatorTool{},
		&financial.PricingOptimizerTool{},
		&financial.BudgetAnalyzerTool{},
		&financial.ROICalculatorTool{},
		&financial.ExpenseCategorizerTool{},
		&financial.BreakEvenAnalyzerTool{},
		&financial.ProfitabilityAnalyzerTool{},
		&financial.FinancialForecastingTool{},
		&financial.PaymentTermsOptimizerTool{},
	}

	for _, tool := range financialTools {
		if err := framework.RegisterTool(tool); err != nil {
			return err
		}
	}

	return nil
}

// RegisterIntegrationTools registers all Integration & External tools
func RegisterIntegrationTools(framework core.Framework) error {
	integrationTools := []tools.Tool{
		&integration.APICallerTool{},
		&integration.FileParserTool{},
		&integration.WebScraperTool{},
		&integration.DatabaseConnectorTool{},
		&integration.DataSyncTool{},
		&integration.WebhookHandlerTool{},
		&integration.EmailSenderTool{},
		&integration.SlackNotifierTool{},
		&integration.CloudStorageTool{},
		&integration.DataExporterTool{},
		&integration.EventStreamProcessorTool{},
		&integration.OAuthAuthenticatorTool{},
	}

	for _, tool := range integrationTools {
		if err := framework.RegisterTool(tool); err != nil {
			return err
		}
	}

	return nil
}

// RegisterCommunicationTools registers all Communication & Collaboration tools
func RegisterCommunicationTools(framework core.Framework) error {
	communicationTools := []tools.Tool{
		&communication.SlackMessageTool{},
		&communication.SlackChannelTool{},
		&communication.TeamsMessageTool{},
		&communication.DiscordMessageTool{},
		&communication.GmailSendTool{},
		&communication.GmailSearchTool{},
		&communication.ZoomMeetingTool{},
		&communication.TwilioSMSTool{},
		&communication.TwilioCallTool{},
	}

	for _, tool := range communicationTools {
		if err := framework.RegisterTool(tool); err != nil {
			return err
		}
	}

	return nil
}

// RegisterProjectManagementTools registers all Project Management tools
func RegisterProjectManagementTools(framework core.Framework) error {
	projectMgmtTools := []tools.Tool{
		&projectmgmt.JiraIssueTool{},
		&projectmgmt.JiraSprintTool{},
		&projectmgmt.AsanaTaskTool{},
		&projectmgmt.AsanaProjectTool{},
		&projectmgmt.TrelloCardTool{},
		&projectmgmt.TrelloBoardTool{},
		&projectmgmt.LinearIssueTool{},
		&projectmgmt.ClickUpTaskTool{},
		&projectmgmt.MondayItemTool{},
	}

	for _, tool := range projectMgmtTools {
		if err := framework.RegisterTool(tool); err != nil {
			return err
		}
	}

	return nil
}
