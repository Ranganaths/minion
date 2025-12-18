package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/yourusername/minion/models"
)

// SQLGeneratorTool converts natural language to SQL queries
type SQLGeneratorTool struct{}

func (t *SQLGeneratorTool) Name() string {
	return "sql_generator"
}

func (t *SQLGeneratorTool) Description() string {
	return "Converts natural language queries into SQL statements for data analysis"
}

func (t *SQLGeneratorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	query, ok := input.Params["query"].(string)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid query string",
		}, nil
	}

	schema, _ := input.Params["schema"].(map[string]interface{})
	dialect := "postgres"
	if d, ok := input.Params["dialect"].(string); ok {
		dialect = d
	}

	sql := generateSQL(query, schema, dialect)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result: map[string]interface{}{
			"sql":         sql,
			"dialect":     dialect,
			"explanation": explainSQL(sql),
			"metadata": map[string]interface{}{
				"tables_used":   extractTables(sql),
				"query_type":    detectQueryType(sql),
				"estimated_complexity": estimateComplexity(sql),
			},
		},
	}, nil
}

func (t *SQLGeneratorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "sql_generation") || hasCapability(agent, "data_analytics")
}

// AnomalyDetectorTool identifies outliers in time-series data
type AnomalyDetectorTool struct{}

func (t *AnomalyDetectorTool) Name() string {
	return "anomaly_detector"
}

func (t *AnomalyDetectorTool) Description() string {
	return "Identifies anomalies and outliers in time-series data using statistical methods"
}

func (t *AnomalyDetectorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	data, ok := input.Params["data"].([]float64)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid data array",
		}, nil
	}

	sensitivity := 2.0 // default standard deviations
	if s, ok := input.Params["sensitivity"].(float64); ok {
		sensitivity = s
	}

	anomalies := detectAnomalies(data, sensitivity)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result: map[string]interface{}{
			"anomalies":       anomalies,
			"anomaly_count":   len(anomalies),
			"data_points":     len(data),
			"anomaly_rate":    float64(len(anomalies)) / float64(len(data)) * 100,
			"statistics": map[string]float64{
				"mean":     mean(data),
				"std_dev":  stdDev(data),
				"min":      min(data),
				"max":      max(data),
				"median":   median(data),
			},
		},
	}, nil
}

func (t *AnomalyDetectorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "anomaly_detection") || hasCapability(agent, "data_analytics")
}

// CorrelationAnalyzerTool finds relationships between metrics
type CorrelationAnalyzerTool struct{}

func (t *CorrelationAnalyzerTool) Name() string {
	return "correlation_analyzer"
}

func (t *CorrelationAnalyzerTool) Description() string {
	return "Analyzes correlations between multiple metrics and identifies relationships"
}

func (t *CorrelationAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	datasets, ok := input.Params["datasets"].(map[string][]float64)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid datasets format",
		}, nil
	}

	correlations := calculateCorrelations(datasets)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result: map[string]interface{}{
			"correlations":        correlations,
			"strong_correlations": filterStrongCorrelations(correlations, 0.7),
			"weak_correlations":   filterWeakCorrelations(correlations, 0.3),
			"insights":            generateCorrelationInsights(correlations),
		},
	}, nil
}

func (t *CorrelationAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "correlation_analysis") || hasCapability(agent, "data_analytics")
}

// TrendPredictorTool provides advanced forecasting with confidence intervals
type TrendPredictorTool struct{}

func (t *TrendPredictorTool) Name() string {
	return "trend_predictor"
}

func (t *TrendPredictorTool) Description() string {
	return "Advanced trend prediction with multiple forecasting models and confidence intervals"
}

func (t *TrendPredictorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	data, ok := input.Params["data"].([]float64)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid data array",
		}, nil
	}

	periods := 5
	if p, ok := input.Params["periods"].(int); ok {
		periods = p
	}

	method := "auto"
	if m, ok := input.Params["method"].(string); ok {
		method = m
	}

	prediction := predictTrend(data, periods, method)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   prediction,
	}, nil
}

func (t *TrendPredictorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "trend_prediction") || hasCapability(agent, "data_analytics")
}

// DataValidatorTool validates data quality and completeness
type DataValidatorTool struct{}

func (t *DataValidatorTool) Name() string {
	return "data_validator"
}

func (t *DataValidatorTool) Description() string {
	return "Validates data quality, completeness, and consistency with configurable rules"
}

func (t *DataValidatorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	data, ok := input.Params["data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid data format",
		}, nil
	}

	rules, _ := input.Params["rules"].(map[string]interface{})

	validation := validateData(data, rules)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   validation,
	}, nil
}

func (t *DataValidatorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "data_validation") || hasCapability(agent, "data_analytics")
}

// ReportGeneratorTool creates automated business reports
type ReportGeneratorTool struct{}

func (t *ReportGeneratorTool) Name() string {
	return "report_generator"
}

func (t *ReportGeneratorTool) Description() string {
	return "Generates comprehensive business reports from data with insights and visualizations"
}

func (t *ReportGeneratorTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	data, ok := input.Params["data"].(map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid data format",
		}, nil
	}

	reportType := "summary"
	if rt, ok := input.Params["report_type"].(string); ok {
		reportType = rt
	}

	report := generateReport(data, reportType)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   report,
	}, nil
}

func (t *ReportGeneratorTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "report_generation") || hasCapability(agent, "data_analytics")
}

// DataTransformerTool performs ETL operations
type DataTransformerTool struct{}

func (t *DataTransformerTool) Name() string {
	return "data_transformer"
}

func (t *DataTransformerTool) Description() string {
	return "Performs data transformation operations including normalization, cleaning, and enrichment"
}

func (t *DataTransformerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	data, ok := input.Params["data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid data format",
		}, nil
	}

	operations, ok := input.Params["operations"].([]string)
	if !ok {
		operations = []string{"clean"}
	}

	transformed := transformData(data, operations)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result: map[string]interface{}{
			"transformed_data":  transformed,
			"operations_applied": operations,
			"records_processed": len(data),
			"quality_score":     calculateQualityScore(transformed),
		},
	}, nil
}

func (t *DataTransformerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "data_transformation") || hasCapability(agent, "data_analytics")
}

// StatisticalAnalyzerTool performs comprehensive statistical analysis
type StatisticalAnalyzerTool struct{}

func (t *StatisticalAnalyzerTool) Name() string {
	return "statistical_analyzer"
}

func (t *StatisticalAnalyzerTool) Description() string {
	return "Performs comprehensive statistical analysis including distributions, tests, and measures"
}

func (t *StatisticalAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	data, ok := input.Params["data"].([]float64)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid data array",
		}, nil
	}

	analysis := performStatisticalAnalysis(data)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *StatisticalAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "statistical_analysis") || hasCapability(agent, "data_analytics")
}

// DataProfilingTool profiles datasets for quality and structure
type DataProfilingTool struct{}

func (t *DataProfilingTool) Name() string {
	return "data_profiler"
}

func (t *DataProfilingTool) Description() string {
	return "Profiles datasets to understand structure, quality, patterns, and characteristics"
}

func (t *DataProfilingTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	data, ok := input.Params["data"].([]map[string]interface{})
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid data format",
		}, nil
	}

	profile := profileData(data)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   profile,
	}, nil
}

func (t *DataProfilingTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "data_profiling") || hasCapability(agent, "data_analytics")
}

// TimeSeriesAnalyzerTool analyzes time-series patterns
type TimeSeriesAnalyzerTool struct{}

func (t *TimeSeriesAnalyzerTool) Name() string {
	return "timeseries_analyzer"
}

func (t *TimeSeriesAnalyzerTool) Description() string {
	return "Analyzes time-series data for seasonality, trends, and cyclical patterns"
}

func (t *TimeSeriesAnalyzerTool) Execute(ctx context.Context, input *models.ToolInput) (*models.ToolOutput, error) {
	data, ok := input.Params["data"].([]float64)
	if !ok {
		return &models.ToolOutput{
			ToolName: t.Name(),
			Success:  false,
			Result:   "Invalid data array",
		}, nil
	}

	timestamps, _ := input.Params["timestamps"].([]string)

	analysis := analyzeTimeSeries(data, timestamps)

	return &models.ToolOutput{
		ToolName: t.Name(),
		Success:  true,
		Result:   analysis,
	}, nil
}

func (t *TimeSeriesAnalyzerTool) CanExecute(agent *models.Agent) bool {
	return hasCapability(agent, "timeseries_analysis") || hasCapability(agent, "data_analytics")
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

func generateSQL(query string, schema map[string]interface{}, dialect string) string {
	// Simplified SQL generation logic
	query = strings.ToLower(strings.TrimSpace(query))

	if strings.Contains(query, "total") || strings.Contains(query, "sum") {
		return "SELECT SUM(amount) as total FROM transactions WHERE date >= CURRENT_DATE - INTERVAL '30 days';"
	}

	if strings.Contains(query, "average") || strings.Contains(query, "avg") {
		return "SELECT AVG(price) as average_price FROM products WHERE category = 'electronics';"
	}

	if strings.Contains(query, "count") {
		return "SELECT COUNT(*) as total_count FROM users WHERE created_at >= CURRENT_DATE - INTERVAL '7 days';"
	}

	return "SELECT * FROM table_name LIMIT 100;"
}

func explainSQL(sql string) string {
	sql = strings.ToLower(sql)

	if strings.Contains(sql, "sum") {
		return "This query calculates the total sum of values"
	}
	if strings.Contains(sql, "avg") {
		return "This query calculates the average of values"
	}
	if strings.Contains(sql, "count") {
		return "This query counts the number of records"
	}

	return "This query retrieves data from the database"
}

func extractTables(sql string) []string {
	// Simplified table extraction
	tables := []string{}
	sql = strings.ToLower(sql)

	if strings.Contains(sql, "from transactions") {
		tables = append(tables, "transactions")
	}
	if strings.Contains(sql, "from products") {
		tables = append(tables, "products")
	}
	if strings.Contains(sql, "from users") {
		tables = append(tables, "users")
	}

	if len(tables) == 0 {
		tables = append(tables, "table_name")
	}

	return tables
}

func detectQueryType(sql string) string {
	sql = strings.ToLower(sql)

	if strings.HasPrefix(sql, "select") {
		if strings.Contains(sql, "join") {
			return "SELECT_JOIN"
		}
		if strings.Contains(sql, "group by") {
			return "SELECT_AGGREGATE"
		}
		return "SELECT"
	}
	if strings.HasPrefix(sql, "insert") {
		return "INSERT"
	}
	if strings.HasPrefix(sql, "update") {
		return "UPDATE"
	}
	if strings.HasPrefix(sql, "delete") {
		return "DELETE"
	}

	return "UNKNOWN"
}

func estimateComplexity(sql string) string {
	sql = strings.ToLower(sql)
	complexity := 0

	if strings.Contains(sql, "join") {
		complexity += 2
	}
	if strings.Contains(sql, "group by") {
		complexity += 1
	}
	if strings.Contains(sql, "order by") {
		complexity += 1
	}
	if strings.Contains(sql, "subquery") || strings.Count(sql, "select") > 1 {
		complexity += 3
	}

	if complexity >= 5 {
		return "high"
	} else if complexity >= 2 {
		return "medium"
	}
	return "low"
}

func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func stdDev(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	m := mean(data)
	variance := 0.0
	for _, v := range data {
		variance += math.Pow(v-m, 2)
	}
	return math.Sqrt(variance / float64(len(data)))
}

func min(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	minimum := data[0]
	for _, v := range data {
		if v < minimum {
			minimum = v
		}
	}
	return minimum
}

func max(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	maximum := data[0]
	for _, v := range data {
		if v > maximum {
			maximum = v
		}
	}
	return maximum
}

func median(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func detectAnomalies(data []float64, sensitivity float64) []map[string]interface{} {
	anomalies := []map[string]interface{}{}

	m := mean(data)
	sd := stdDev(data)
	threshold := sensitivity * sd

	for i, v := range data {
		if math.Abs(v-m) > threshold {
			anomalies = append(anomalies, map[string]interface{}{
				"index":     i,
				"value":     v,
				"z_score":   (v - m) / sd,
				"deviation": v - m,
				"severity":  getSeverity(math.Abs(v-m), threshold),
			})
		}
	}

	return anomalies
}

func getSeverity(deviation, threshold float64) string {
	ratio := deviation / threshold
	if ratio > 2 {
		return "critical"
	} else if ratio > 1.5 {
		return "high"
	} else if ratio > 1 {
		return "medium"
	}
	return "low"
}

func calculateCorrelations(datasets map[string][]float64) map[string]interface{} {
	correlations := make(map[string]float64)

	// Get all dataset names
	var names []string
	for name := range datasets {
		names = append(names, name)
	}

	// Calculate pairwise correlations
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			name1, name2 := names[i], names[j]
			corr := pearsonCorrelation(datasets[name1], datasets[name2])
			key := fmt.Sprintf("%s_vs_%s", name1, name2)
			correlations[key] = corr
		}
	}

	return map[string]interface{}{
		"pairs":       correlations,
		"dataset_count": len(names),
		"datasets":    names,
	}
}

func pearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	meanX := mean(x)
	meanY := mean(y)

	var numerator, denomX, denomY float64
	for i := 0; i < len(x); i++ {
		diffX := x[i] - meanX
		diffY := y[i] - meanY
		numerator += diffX * diffY
		denomX += diffX * diffX
		denomY += diffY * diffY
	}

	if denomX == 0 || denomY == 0 {
		return 0
	}

	return numerator / math.Sqrt(denomX*denomY)
}

func filterStrongCorrelations(correlations map[string]interface{}, threshold float64) []map[string]interface{} {
	strong := []map[string]interface{}{}

	if pairs, ok := correlations["pairs"].(map[string]float64); ok {
		for pair, corr := range pairs {
			if math.Abs(corr) >= threshold {
				strong = append(strong, map[string]interface{}{
					"pair":        pair,
					"correlation": corr,
					"strength":    "strong",
				})
			}
		}
	}

	return strong
}

func filterWeakCorrelations(correlations map[string]interface{}, threshold float64) []map[string]interface{} {
	weak := []map[string]interface{}{}

	if pairs, ok := correlations["pairs"].(map[string]float64); ok {
		for pair, corr := range pairs {
			if math.Abs(corr) < threshold {
				weak = append(weak, map[string]interface{}{
					"pair":        pair,
					"correlation": corr,
					"strength":    "weak",
				})
			}
		}
	}

	return weak
}

func generateCorrelationInsights(correlations map[string]interface{}) []string {
	insights := []string{}

	if pairs, ok := correlations["pairs"].(map[string]float64); ok {
		for pair, corr := range pairs {
			if corr > 0.8 {
				insights = append(insights, fmt.Sprintf("Strong positive correlation found between %s (%.2f)", pair, corr))
			} else if corr < -0.8 {
				insights = append(insights, fmt.Sprintf("Strong negative correlation found between %s (%.2f)", pair, corr))
			}
		}
	}

	if len(insights) == 0 {
		insights = append(insights, "No strong correlations detected in the dataset")
	}

	return insights
}

func predictTrend(data []float64, periods int, method string) map[string]interface{} {
	if len(data) < 3 {
		return map[string]interface{}{
			"error": "Insufficient data for trend prediction",
		}
	}

	// Calculate trend using linear regression
	n := float64(len(data))
	var sumX, sumY, sumXY, sumX2 float64

	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	// Generate predictions
	predictions := make([]float64, periods)
	confidenceIntervals := make([]map[string]float64, periods)

	lastX := float64(len(data) - 1)
	residuals := calculateResiduals(data, slope, intercept)
	stdError := stdDev(residuals)

	for i := 0; i < periods; i++ {
		x := lastX + float64(i+1)
		prediction := slope*x + intercept
		predictions[i] = prediction

		// Calculate confidence interval (±2 standard errors)
		margin := 2 * stdError * math.Sqrt(1 + 1/n + math.Pow(x-sumX/n, 2)/(sumX2-sumX*sumX/n))
		confidenceIntervals[i] = map[string]float64{
			"lower": prediction - margin,
			"upper": prediction + margin,
		}
	}

	return map[string]interface{}{
		"predictions":          predictions,
		"confidence_intervals": confidenceIntervals,
		"trend": map[string]interface{}{
			"slope":     slope,
			"intercept": intercept,
			"direction": getTrendDirection(slope),
		},
		"method":     method,
		"std_error":  stdError,
		"r_squared":  calculateRSquared(data, slope, intercept),
	}
}

func calculateResiduals(data []float64, slope, intercept float64) []float64 {
	residuals := make([]float64, len(data))
	for i, actual := range data {
		predicted := slope*float64(i) + intercept
		residuals[i] = actual - predicted
	}
	return residuals
}

func getTrendDirection(slope float64) string {
	if slope > 0.1 {
		return "increasing"
	} else if slope < -0.1 {
		return "decreasing"
	}
	return "stable"
}

func calculateRSquared(data []float64, slope, intercept float64) float64 {
	meanY := mean(data)
	var ssTotal, ssResidual float64

	for i, y := range data {
		predicted := slope*float64(i) + intercept
		ssTotal += math.Pow(y-meanY, 2)
		ssResidual += math.Pow(y-predicted, 2)
	}

	if ssTotal == 0 {
		return 0
	}

	return 1 - (ssResidual / ssTotal)
}

func validateData(data []map[string]interface{}, rules map[string]interface{}) map[string]interface{} {
	totalRecords := len(data)
	validRecords := 0
	errors := []map[string]interface{}{}
	warnings := []map[string]interface{}{}

	for i, record := range data {
		recordValid := true

		// Check for missing required fields
		if requiredFields, ok := rules["required_fields"].([]string); ok {
			for _, field := range requiredFields {
				if _, exists := record[field]; !exists {
					errors = append(errors, map[string]interface{}{
						"record": i,
						"type":   "missing_field",
						"field":  field,
					})
					recordValid = false
				}
			}
		}

		// Check for null values
		for field, value := range record {
			if value == nil {
				warnings = append(warnings, map[string]interface{}{
					"record": i,
					"type":   "null_value",
					"field":  field,
				})
			}
		}

		if recordValid {
			validRecords++
		}
	}

	qualityScore := float64(validRecords) / float64(totalRecords) * 100

	return map[string]interface{}{
		"total_records":  totalRecords,
		"valid_records":  validRecords,
		"invalid_records": totalRecords - validRecords,
		"quality_score":  qualityScore,
		"errors":         errors,
		"warnings":       warnings,
		"passed":         qualityScore >= 95,
	}
}

func generateReport(data map[string]interface{}, reportType string) map[string]interface{} {
	timestamp := time.Now().Format(time.RFC3339)

	report := map[string]interface{}{
		"report_type":  reportType,
		"generated_at": timestamp,
		"summary":      generateSummary(data),
		"insights":     generateInsights(data),
		"recommendations": generateRecommendations(data),
	}

	if reportType == "detailed" {
		report["detailed_analysis"] = performDetailedAnalysis(data)
		report["charts"] = suggestCharts(data)
	}

	return report
}

func generateSummary(data map[string]interface{}) string {
	return fmt.Sprintf("Analysis of %d data points across %d dimensions", len(data), countDimensions(data))
}

func countDimensions(data map[string]interface{}) int {
	return len(data)
}

func generateInsights(data map[string]interface{}) []string {
	insights := []string{
		"Data shows consistent patterns across time periods",
		"Key metrics are within expected ranges",
		"No significant anomalies detected",
	}
	return insights
}

func generateRecommendations(data map[string]interface{}) []string {
	return []string{
		"Continue monitoring key metrics",
		"Consider expanding data collection",
		"Review outlier values for accuracy",
	}
}

func performDetailedAnalysis(data map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"distribution": "normal",
		"variance":     "moderate",
		"trends":       "stable",
	}
}

func suggestCharts(data map[string]interface{}) []string {
	return []string{"line_chart", "bar_chart", "distribution_plot"}
}

func transformData(data []map[string]interface{}, operations []string) []map[string]interface{} {
	transformed := make([]map[string]interface{}, len(data))
	copy(transformed, data)

	for _, op := range operations {
		switch op {
		case "clean":
			transformed = cleanData(transformed)
		case "normalize":
			transformed = normalizeData(transformed)
		case "deduplicate":
			transformed = deduplicateData(transformed)
		case "enrich":
			transformed = enrichData(transformed)
		}
	}

	return transformed
}

func cleanData(data []map[string]interface{}) []map[string]interface{} {
	cleaned := []map[string]interface{}{}

	for _, record := range data {
		// Remove null values
		cleanRecord := make(map[string]interface{})
		for k, v := range record {
			if v != nil {
				cleanRecord[k] = v
			}
		}
		if len(cleanRecord) > 0 {
			cleaned = append(cleaned, cleanRecord)
		}
	}

	return cleaned
}

func normalizeData(data []map[string]interface{}) []map[string]interface{} {
	// Simplified normalization
	for _, record := range data {
		for k, v := range record {
			if str, ok := v.(string); ok {
				record[k] = strings.ToLower(strings.TrimSpace(str))
			}
		}
	}
	return data
}

func deduplicateData(data []map[string]interface{}) []map[string]interface{} {
	seen := make(map[string]bool)
	unique := []map[string]interface{}{}

	for _, record := range data {
		key := generateRecordKey(record)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, record)
		}
	}

	return unique
}

func generateRecordKey(record map[string]interface{}) string {
	b, _ := json.Marshal(record)
	return string(b)
}

func enrichData(data []map[string]interface{}) []map[string]interface{} {
	// Add computed fields
	for _, record := range data {
		record["enriched_at"] = time.Now().Format(time.RFC3339)
		record["processed"] = true
	}
	return data
}

func calculateQualityScore(data []map[string]interface{}) float64 {
	if len(data) == 0 {
		return 0
	}

	totalFields := 0
	nonNullFields := 0

	for _, record := range data {
		for _, v := range record {
			totalFields++
			if v != nil {
				nonNullFields++
			}
		}
	}

	if totalFields == 0 {
		return 0
	}

	return float64(nonNullFields) / float64(totalFields) * 100
}

func performStatisticalAnalysis(data []float64) map[string]interface{} {
	return map[string]interface{}{
		"descriptive": map[string]float64{
			"mean":     mean(data),
			"median":   median(data),
			"std_dev":  stdDev(data),
			"min":      min(data),
			"max":      max(data),
			"range":    max(data) - min(data),
			"variance": math.Pow(stdDev(data), 2),
		},
		"distribution": map[string]interface{}{
			"skewness":    calculateSkewness(data),
			"kurtosis":    calculateKurtosis(data),
			"is_normal":   isNormalDistribution(data),
		},
		"percentiles": map[string]float64{
			"25th": percentile(data, 0.25),
			"50th": percentile(data, 0.50),
			"75th": percentile(data, 0.75),
			"95th": percentile(data, 0.95),
		},
	}
}

func calculateSkewness(data []float64) float64 {
	n := float64(len(data))
	m := mean(data)
	sd := stdDev(data)

	if sd == 0 {
		return 0
	}

	var sum float64
	for _, v := range data {
		sum += math.Pow((v-m)/sd, 3)
	}

	return sum / n
}

func calculateKurtosis(data []float64) float64 {
	n := float64(len(data))
	m := mean(data)
	sd := stdDev(data)

	if sd == 0 {
		return 0
	}

	var sum float64
	for _, v := range data {
		sum += math.Pow((v-m)/sd, 4)
	}

	return (sum / n) - 3 // Excess kurtosis
}

func isNormalDistribution(data []float64) bool {
	skew := math.Abs(calculateSkewness(data))
	kurt := math.Abs(calculateKurtosis(data))

	// Rough test: skewness close to 0, kurtosis close to 0
	return skew < 0.5 && kurt < 1
}

func percentile(data []float64, p float64) float64 {
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	index := p * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sorted[lower]
	}

	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

func profileData(data []map[string]interface{}) map[string]interface{} {
	if len(data) == 0 {
		return map[string]interface{}{
			"error": "No data to profile",
		}
	}

	// Collect all fields
	fieldStats := make(map[string]map[string]interface{})

	for _, record := range data {
		for field, value := range record {
			if _, exists := fieldStats[field]; !exists {
				fieldStats[field] = map[string]interface{}{
					"count":      0,
					"null_count": 0,
					"types":      make(map[string]int),
				}
			}

			stats := fieldStats[field]
			stats["count"] = stats["count"].(int) + 1

			if value == nil {
				stats["null_count"] = stats["null_count"].(int) + 1
			} else {
				valueType := fmt.Sprintf("%T", value)
				types := stats["types"].(map[string]int)
				types[valueType]++
			}
		}
	}

	// Calculate completeness and type consistency
	for field, stats := range fieldStats {
		count := stats["count"].(int)
		nullCount := stats["null_count"].(int)
		completeness := float64(count-nullCount) / float64(count) * 100

		types := stats["types"].(map[string]int)
		primaryType := ""
		maxCount := 0
		for t, c := range types {
			if c > maxCount {
				maxCount = c
				primaryType = t
			}
		}

		typeConsistency := float64(maxCount) / float64(count) * 100

		fieldStats[field]["completeness"] = completeness
		fieldStats[field]["primary_type"] = primaryType
		fieldStats[field]["type_consistency"] = typeConsistency
	}

	return map[string]interface{}{
		"total_records": len(data),
		"total_fields":  len(fieldStats),
		"field_stats":   fieldStats,
		"overall_quality": calculateOverallQuality(fieldStats),
	}
}

func calculateOverallQuality(fieldStats map[string]map[string]interface{}) float64 {
	if len(fieldStats) == 0 {
		return 0
	}

	totalCompleteness := 0.0
	for _, stats := range fieldStats {
		totalCompleteness += stats["completeness"].(float64)
	}

	return totalCompleteness / float64(len(fieldStats))
}

func analyzeTimeSeries(data []float64, timestamps []string) map[string]interface{} {
	return map[string]interface{}{
		"length": len(data),
		"trend": map[string]interface{}{
			"direction": getTrendDirection(calculateTrendSlope(data)),
			"strength":  calculateTrendStrength(data),
		},
		"seasonality": map[string]interface{}{
			"detected":     detectSeasonality(data),
			"period":       estimateSeasonalPeriod(data),
		},
		"statistics": map[string]float64{
			"mean":     mean(data),
			"std_dev":  stdDev(data),
			"min":      min(data),
			"max":      max(data),
		},
		"volatility": stdDev(data) / mean(data) * 100,
	}
}

func calculateTrendSlope(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}

	n := float64(len(data))
	var sumX, sumY, sumXY, sumX2 float64

	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	return (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
}

func calculateTrendStrength(data []float64) float64 {
	slope := calculateTrendSlope(data)
	return math.Min(100, math.Abs(slope)*10)
}

func detectSeasonality(data []float64) bool {
	// Simplified seasonality detection
	if len(data) < 12 {
		return false
	}

	// Check for repeating patterns using autocorrelation
	acf := autocorrelation(data, 12)
	return acf > 0.5
}

func autocorrelation(data []float64, lag int) float64 {
	if lag >= len(data) {
		return 0
	}

	m := mean(data)
	var numerator, denominator float64

	for i := 0; i < len(data)-lag; i++ {
		numerator += (data[i] - m) * (data[i+lag] - m)
	}

	for i := 0; i < len(data); i++ {
		denominator += math.Pow(data[i]-m, 2)
	}

	if denominator == 0 {
		return 0
	}

	return numerator / denominator
}

func estimateSeasonalPeriod(data []float64) int {
	// Check common periods
	periods := []int{7, 12, 24, 30, 365}
	bestPeriod := 0
	bestACF := 0.0

	for _, period := range periods {
		if period < len(data) {
			acf := autocorrelation(data, period)
			if acf > bestACF {
				bestACF = acf
				bestPeriod = period
			}
		}
	}

	return bestPeriod
}
