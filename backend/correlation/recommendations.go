package correlation

import (
	"github.com/sennet/sennet/backend/db"
)

type RecommendationType string

const (
	RecCrossAZ       RecommendationType = "cross_az_traffic"
	RecCrossRegionS3 RecommendationType = "cross_region_s3"
	RecNATGateway    RecommendationType = "nat_gateway_abuse"
	RecVPCEndpoint   RecommendationType = "use_vpc_endpoint"
)

type RecommendationRule struct {
	Type        RecommendationType
	Description string
	Condition   func(costs []db.EgressCost) bool
	Savings     func(costs []db.EgressCost) float64
}

var DefaultRules = []RecommendationRule{
	{
		Type:        RecCrossAZ,
		Description: "Move replicas to same Availability Zone to reduce cross-AZ data transfer costs",
		Condition: func(costs []db.EgressCost) bool {
			for _, c := range costs {
				if c.Service == "AmazonEC2" && c.CostUSD > 100 {
					return true
				}
			}
			return false
		},
		Savings: func(costs []db.EgressCost) float64 {
			var total float64
			for _, c := range costs {
				if c.Service == "AmazonEC2" {
					total += c.CostUSD * 0.5
				}
			}
			return total
		},
	},
	{
		Type:        RecVPCEndpoint,
		Description: "Use VPC Endpoints for AWS services (S3, DynamoDB) to eliminate NAT Gateway charges",
		Condition: func(costs []db.EgressCost) bool {
			for _, c := range costs {
				if c.Service == "AmazonEC2" && c.CostUSD > 50 {
					return true
				}
			}
			return false
		},
		Savings: func(costs []db.EgressCost) float64 {
			var total float64
			for _, c := range costs {
				if c.Service == "AmazonEC2" {
					total += c.CostUSD * 0.3
				}
			}
			return total
		},
	},
	{
		Type:        RecCrossRegionS3,
		Description: "Use S3 buckets in the same region as your compute resources",
		Condition: func(costs []db.EgressCost) bool {
			for _, c := range costs {
				if c.Service == "AmazonS3" && c.CostUSD > 20 {
					return true
				}
			}
			return false
		},
		Savings: func(costs []db.EgressCost) float64 {
			var total float64
			for _, c := range costs {
				if c.Service == "AmazonS3" {
					total += c.CostUSD * 0.8
				}
			}
			return total
		},
	},
}

type RecommendationEngine struct {
	database *db.DB
	rules    []RecommendationRule
}

func NewRecommendationEngine(database *db.DB) *RecommendationEngine {
	return &RecommendationEngine{
		database: database,
		rules:    DefaultRules,
	}
}

func (e *RecommendationEngine) GenerateRecommendations(startDate, endDate string) error {
	costs, err := e.database.GetEgressCosts(startDate, endDate)
	if err != nil {
		return err
	}

	for _, rule := range e.rules {
		if rule.Condition(costs) {
			savings := rule.Savings(costs)
			if savings > 0 {
				e.database.SaveRecommendation(
					string(rule.Type),
					rule.Description,
					savings,
				)
			}
		}
	}

	return nil
}
