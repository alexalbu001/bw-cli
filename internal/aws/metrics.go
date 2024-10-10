// File: internal/aws/metrics.go

package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type ServiceMetrics struct {
	CPUUtilization    float64
	MemoryUtilization float64
}

func getServiceMetrics(ctx context.Context, cwClient *cloudwatch.Client, cluster, serviceName string) (*ServiceMetrics, error) {
	endTime := time.Now()
	startTime := endTime.Add(-5 * time.Minute)

	cpuUtilization, err := getMetric(ctx, cwClient, "AWS/ECS", "CPUUtilization", cluster, serviceName, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("error fetching CPUUtilization: %v", err)
	}

	memoryUtilization, err := getMetric(ctx, cwClient, "AWS/ECS", "MemoryUtilization", cluster, serviceName, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("error fetching MemoryUtilization: %v", err)
	}

	return &ServiceMetrics{
		CPUUtilization:    *cpuUtilization,
		MemoryUtilization: *memoryUtilization,
	}, nil
}

func getMetric(ctx context.Context, cwClient *cloudwatch.Client, namespace, metricName, cluster, serviceName string, startTime, endTime time.Time) (*float64, error) {
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		StartTime:  aws.Time(startTime),
		EndTime:    aws.Time(endTime),
		Period:     aws.Int32(300), // 5 minutes
		Statistics: []types.Statistic{types.StatisticAverage},
		Dimensions: []types.Dimension{
			{
				Name:  aws.String("ClusterName"),
				Value: aws.String(cluster),
			},
			{
				Name:  aws.String("ServiceName"),
				Value: aws.String(serviceName),
			},
		},
	}

	output, err := cwClient.GetMetricStatistics(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric %s: %v", metricName, err)
	}

	if len(output.Datapoints) == 0 {
		return aws.Float64(0), nil
	}

	return output.Datapoints[0].Average, nil
}
