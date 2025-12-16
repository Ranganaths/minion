package domains

import (
	"github.com/agentql/agentql/pkg/minion/core"
	"github.com/agentql/agentql/pkg/minion/tools"
	"github.com/agentql/agentql/pkg/minion/tools/domains/marketing"
	"github.com/agentql/agentql/pkg/minion/tools/domains/sales"
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
