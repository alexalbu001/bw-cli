package aws

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/alexalbu001/bw-cli/pkg"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

const maxDescribeServicesBatchSize = 10

// ECSClientAPI defines the interface for ECS client operations
type ECSClientAPI interface {
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
}

// GetAllServiceDetails fetches services with running and desired count details from all clusters in parallel.
func GetAllServiceDetails(ctx context.Context, ecsClient ECSClientAPI, cwClient *cloudwatch.Client) ([]pkg.ServiceDetails, error) {
	clusters, err := listClusters(ctx, ecsClient)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	serviceCh := make(chan []pkg.ServiceDetails, len(clusters))

	for _, cluster := range clusters {
		wg.Add(1)
		go func(cluster string) {
			defer wg.Done()
			services, err := describeServicesInBatches(cluster, ctx, ecsClient, cwClient)
			if err != nil {
				log.Printf("Error describing services for cluster %s: %v", cluster, err)
				return
			}
			serviceCh <- services
		}(cluster)
	}

	wg.Wait()
	close(serviceCh)

	var allServices []pkg.ServiceDetails
	for services := range serviceCh {
		allServices = append(allServices, services...)
	}

	return allServices, nil
}

func GetServiceDetails(ctx context.Context, ecsClient ECSClientAPI, cwClient *cloudwatch.Client, serviceName, cluster string) (pkg.ServiceDetails, error) {
	input := &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{serviceName},
	}

	output, err := ecsClient.DescribeServices(ctx, input)
	if err != nil {
		return pkg.ServiceDetails{}, fmt.Errorf("error describing service %s: %v", serviceName, err)
	}

	if len(output.Services) == 0 {
		return pkg.ServiceDetails{}, fmt.Errorf("no service details found for service %s", serviceName)
	}

	service := output.Services[0]

	metrics, err := getServiceMetrics(ctx, cwClient, cluster, serviceName)
	if err != nil {
		log.Printf("Error fetching metrics for service %s: %v", serviceName, err)
		metrics = &ServiceMetrics{CPUUtilization: 0, MemoryUtilization: 0}
	}

	return pkg.ServiceDetails{
		ServiceName:       *service.ServiceName,
		RunningCount:      int64(service.RunningCount),
		DesiredCount:      int64(service.DesiredCount),
		Status:            *service.Status,
		Cluster:           cluster,
		CPUUtilization:    metrics.CPUUtilization,
		MemoryUtilization: metrics.MemoryUtilization,
	}, nil
}

func listClusters(ctx context.Context, ecsClient ECSClientAPI) ([]string, error) {
	input := &ecs.ListClustersInput{}
	var clusterArns []string

	paginator := ecs.NewListClustersPaginator(ecsClient, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		clusterArns = append(clusterArns, output.ClusterArns...)
	}
	return clusterArns, nil
}

func listServices(ctx context.Context, ecsClient ECSClientAPI, cluster string) ([]string, error) {
	input := &ecs.ListServicesInput{
		Cluster: &cluster,
	}
	var serviceArns []string

	paginator := ecs.NewListServicesPaginator(ecsClient, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		serviceArns = append(serviceArns, output.ServiceArns...)
	}

	return serviceArns, nil
}

// describeServicesInBatches describes services for a given cluster in batches.
func describeServicesInBatches(cluster string, ctx context.Context, ecsClient ECSClientAPI, cwClient *cloudwatch.Client) ([]pkg.ServiceDetails, error) {
	serviceArns, err := listServices(ctx, ecsClient, cluster)
	if err != nil || len(serviceArns) == 0 {
		return nil, err
	}

	var services []pkg.ServiceDetails
	for i := 0; i < len(serviceArns); i += maxDescribeServicesBatchSize {
		end := i + maxDescribeServicesBatchSize
		if end > len(serviceArns) {
			end = len(serviceArns)
		}

		batch := serviceArns[i:end]
		input := &ecs.DescribeServicesInput{
			Cluster:  &cluster,
			Services: batch,
		}

		output, err := ecsClient.DescribeServices(ctx, input)
		if err != nil {
			fmt.Printf("Error describing services in cluster %s: %v\n", cluster, err)
			continue
		}

		for _, service := range output.Services {
			metrics, err := getServiceMetrics(ctx, cwClient, cluster, *service.ServiceName)
			if err != nil {
				log.Printf("Error fetching metrics for service %s: %v", *service.ServiceName, err)
				metrics = &ServiceMetrics{CPUUtilization: 0, MemoryUtilization: 0}
			}

			services = append(services, pkg.ServiceDetails{
				ServiceName:       *service.ServiceName,
				RunningCount:      int64(service.RunningCount),
				DesiredCount:      int64(service.DesiredCount),
				Status:            *service.Status,
				Cluster:           cluster,
				CPUUtilization:    metrics.CPUUtilization,
				MemoryUtilization: metrics.MemoryUtilization,
			})
		}
	}

	return services, nil
}

// UpdateServiceDesiredCount updates the desired count for a given ECS service.
func UpdateServiceDesiredCount(ctx context.Context, ecsClient ECSClientAPI, serviceName, cluster string, desiredCount int64) error {
	input := &ecs.UpdateServiceInput{
		Cluster:      &cluster,
		Service:      &serviceName,
		DesiredCount: aws.Int32(int32(desiredCount)),
	}

	_, err := ecsClient.UpdateService(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update service %s in cluster %s: %v", serviceName, cluster, err)
	}
	return nil
}

// RestartService forces a redeploy of the ECS service by calling the update-service command.
func RestartService(ctx context.Context, ecsClient ECSClientAPI, serviceName, cluster string) error {
	input := &ecs.UpdateServiceInput{
		Cluster:            &cluster,
		Service:            &serviceName,
		ForceNewDeployment: true,
	}

	_, err := ecsClient.UpdateService(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %v", serviceName, err)
	}

	return nil
}

// GetServiceDeploymentStatus fetches the deployment status of a specific ECS service.
func GetServiceDeploymentStatus(ctx context.Context, ecsClient ECSClientAPI, serviceName, cluster string) (string, error) {
	input := &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{serviceName},
	}

	output, err := ecsClient.DescribeServices(ctx, input)
	if err != nil {
		return "", fmt.Errorf("error describing service %s: %v", serviceName, err)
	}

	if len(output.Services) == 0 || len(output.Services[0].Deployments) == 0 {
		return "Unknown", nil
	}

	deployment := output.Services[0].Deployments[0]
	switch deployment.RolloutState {
	case "IN_PROGRESS":
		return fmt.Sprintf("Deploying (%d/%d)", deployment.RunningCount, deployment.DesiredCount), nil
	case "COMPLETED":
		if deployment.RunningCount == deployment.DesiredCount {
			return "Stable", nil
		}
	case "FAILED":
		return "Deployment Failed", nil
	}
	return *deployment.Status, nil
}

// GetTaskArnForService fetches the ARN of the running task for the specified service.
func GetTaskArnForService(ctx context.Context, ecsClient ECSClientAPI, cluster, serviceName string) (string, error) {
	input := &ecs.ListTasksInput{
		Cluster:     &cluster,
		ServiceName: &serviceName,
	}

	output, err := ecsClient.ListTasks(ctx, input)
	if err != nil {
		return "", fmt.Errorf("error listing tasks for service %s: %v", serviceName, err)
	}

	if len(output.TaskArns) == 0 {
		return "", fmt.Errorf("no running tasks found for service %s", serviceName)
	}

	return output.TaskArns[0], nil
}

// PollServiceUpdates continuously polls for updates to the given services and sends updates through a channel.
func PollServiceUpdates(ctx context.Context, ecsClient ECSClientAPI, cwClient *cloudwatch.Client, updateInterval time.Duration) chan []pkg.ServiceDetails {
	updates := make(chan []pkg.ServiceDetails)

	go func() {
		ticker := time.NewTicker(updateInterval)
		defer ticker.Stop()
		defer close(updates)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				services, err := GetAllServiceDetails(ctx, ecsClient, cwClient)
				if err != nil {
					log.Printf("Error fetching service details: %v", err)
					continue
				}
				updates <- services
			}
		}
	}()

	return updates
}
