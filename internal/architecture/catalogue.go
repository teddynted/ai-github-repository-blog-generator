package architecture

import "strings"

// serviceInfo describes how an AWS service is categorized and drawn. Icon is the
// canonical AWS Architecture Icons path (referenced, not embedded — the icons are
// AWS assets a downstream renderer supplies).
type serviceInfo struct {
	Canonical string
	Category  string // Compute | Serverless | Storage | Database | Messaging | Integration | Networking | Security | Observability | AI/ML | Other
	Icon      string
	Color     string // hex fill for SVG nodes
}

// categoryOrder is the left-to-right column order for layered layouts.
var categoryOrder = []string{
	"Networking", "Security", "Compute", "Serverless", "AI/ML",
	"Integration", "Messaging", "Database", "Storage", "Observability", "Other",
}

var categoryColor = map[string]string{
	"Networking":    "#8C4FFF",
	"Security":      "#DD344C",
	"Compute":       "#ED7100",
	"Serverless":    "#ED7100",
	"AI/ML":         "#01A88D",
	"Integration":   "#E7157B",
	"Messaging":     "#E7157B",
	"Database":      "#2E27AD",
	"Storage":       "#7AA116",
	"Observability": "#E7157B",
	"Other":         "#232F3E",
}

// catalogue maps a lowercased match key to service info. Matching is by
// substring so context strings like "AWS Lambda" or "Amazon SQS" resolve.
var catalogue = []struct {
	key  string
	info serviceInfo
}{
	{"lambda", serviceInfo{"AWS Lambda", "Serverless", "Compute/AWS-Lambda", "#ED7100"}},
	{"fargate", serviceInfo{"AWS Fargate", "Serverless", "Compute/AWS-Fargate", "#ED7100"}},
	{"ec2", serviceInfo{"Amazon EC2", "Compute", "Compute/Amazon-EC2", "#ED7100"}},
	{"ecs", serviceInfo{"Amazon ECS", "Compute", "Containers/Amazon-Elastic-Container-Service", "#ED7100"}},
	{"eks", serviceInfo{"Amazon EKS", "Compute", "Containers/Amazon-Elastic-Kubernetes-Service", "#ED7100"}},
	{"step functions", serviceInfo{"AWS Step Functions", "Integration", "App-Integration/AWS-Step-Functions", "#E7157B"}},
	{"eventbridge", serviceInfo{"Amazon EventBridge", "Integration", "App-Integration/Amazon-EventBridge", "#E7157B"}},
	{"api gateway", serviceInfo{"Amazon API Gateway", "Networking", "Networking-Content-Delivery/Amazon-API-Gateway", "#8C4FFF"}},
	{"sqs", serviceInfo{"Amazon SQS", "Messaging", "App-Integration/Amazon-Simple-Queue-Service", "#E7157B"}},
	{"sns", serviceInfo{"Amazon SNS", "Messaging", "App-Integration/Amazon-Simple-Notification-Service", "#E7157B"}},
	{"s3", serviceInfo{"Amazon S3", "Storage", "Storage/Amazon-Simple-Storage-Service", "#7AA116"}},
	{"dynamodb", serviceInfo{"Amazon DynamoDB", "Database", "Database/Amazon-DynamoDB", "#2E27AD"}},
	{"rds", serviceInfo{"Amazon RDS", "Database", "Database/Amazon-RDS", "#2E27AD"}},
	{"aurora", serviceInfo{"Amazon Aurora", "Database", "Database/Amazon-Aurora", "#2E27AD"}},
	{"bedrock", serviceInfo{"Amazon Bedrock", "AI/ML", "Machine-Learning/Amazon-Bedrock", "#01A88D"}},
	{"sagemaker", serviceInfo{"Amazon SageMaker", "AI/ML", "Machine-Learning/Amazon-SageMaker", "#01A88D"}},
	{"cloudwatch", serviceInfo{"Amazon CloudWatch", "Observability", "Management-Governance/Amazon-CloudWatch", "#E7157B"}},
	{"cloudformation", serviceInfo{"AWS CloudFormation", "Integration", "Management-Governance/AWS-CloudFormation", "#E7157B"}},
	{"cloudfront", serviceInfo{"Amazon CloudFront", "Networking", "Networking-Content-Delivery/Amazon-CloudFront", "#8C4FFF"}},
	{"route 53", serviceInfo{"Amazon Route 53", "Networking", "Networking-Content-Delivery/Amazon-Route-53", "#8C4FFF"}},
	{"route53", serviceInfo{"Amazon Route 53", "Networking", "Networking-Content-Delivery/Amazon-Route-53", "#8C4FFF"}},
	{"load balancer", serviceInfo{"Application Load Balancer", "Networking", "Networking-Content-Delivery/Elastic-Load-Balancing", "#8C4FFF"}},
	{"nat gateway", serviceInfo{"NAT Gateway", "Networking", "Networking-Content-Delivery/Amazon-VPC-NAT-Gateway", "#8C4FFF"}},
	{"vpc", serviceInfo{"Amazon VPC", "Networking", "Networking-Content-Delivery/Amazon-Virtual-Private-Cloud", "#8C4FFF"}},
	{"security group", serviceInfo{"Security Groups", "Security", "Security-Identity-Compliance/AWS-Identity-and-Access-Management", "#DD344C"}},
	{"secrets manager", serviceInfo{"AWS Secrets Manager", "Security", "Security-Identity-Compliance/AWS-Secrets-Manager", "#DD344C"}},
	{"iam", serviceInfo{"AWS IAM", "Security", "Security-Identity-Compliance/AWS-Identity-and-Access-Management", "#DD344C"}},
	{"cognito", serviceInfo{"Amazon Cognito", "Security", "Security-Identity-Compliance/Amazon-Cognito", "#DD344C"}},
	{"kinesis", serviceInfo{"Amazon Kinesis", "Integration", "Analytics/Amazon-Kinesis", "#E7157B"}},
}

// lookupService resolves a context service string to its catalogue info. The
// second return is false when the service is not recognized (it is still drawn,
// as "Other", so nothing grounded is dropped).
func lookupService(name string) (serviceInfo, bool) {
	l := strings.ToLower(collapse(name))
	// Longest key first so "api gateway" beats "gateway" etc.
	for _, entry := range catalogueByKeyLen {
		if strings.Contains(l, entry.key) {
			return entry.info, true
		}
	}
	return serviceInfo{Canonical: collapse(name), Category: "Other", Color: categoryColor["Other"]}, false
}

// catalogueByKeyLen is the catalogue sorted longest-key-first for greedy matching.
var catalogueByKeyLen = sortByKeyLen(catalogue)

func sortByKeyLen(in []struct {
	key  string
	info serviceInfo
}) []struct {
	key  string
	info serviceInfo
} {
	out := append([]struct {
		key  string
		info serviceInfo
	}{}, in...)
	// Simple insertion sort by descending key length (small, stable).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && len(out[j].key) > len(out[j-1].key); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
