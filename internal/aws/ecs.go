package aws

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/alexalbu001/bw-cli/pkg"
	"github.com/aws/aws-sdk-go-v2/aws"
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

// Service Listing and Description
// -------------------------------

func GetAllServiceDetails(ctx context.Context, ecsClient ECSClientAPI) ([]pkg.ServiceDetails, error) {
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
			services, err := describeServicesInBatches(cluster, ctx, ecsClient)
			if err != nil {
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

func GetServiceDetails(ctx context.Context, ecsClient ECSClientAPI, serviceName, cluster string) (pkg.ServiceDetails, error) {
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
	return pkg.ServiceDetails{
		ServiceName:  *service.ServiceName,
		RunningCount: int64(service.RunningCount),
		DesiredCount: int64(service.DesiredCount),
		Status:       *service.Status,
		Cluster:      cluster,
	}, nil
}

// Helper functions for listing and describing
// -------------------------------------------

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

func describeServicesInBatches(cluster string, ctx context.Context, ecsClient ECSClientAPI) ([]pkg.ServiceDetails, error) {
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
			services = append(services, pkg.ServiceDetails{
				ServiceName:  *service.ServiceName,
				RunningCount: int64(service.RunningCount),
				DesiredCount: int64(service.DesiredCount),
				Status:       *service.Status,
				Cluster:      cluster,
			})
		}
	}

	return services, nil
}

// Service Management Operations
// -----------------------------

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

// Deployment Status
// -----------------

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

// Container Operations
// --------------------

func ExecCommandToContainer(cluster, task, container, command string) error {
	fmt.Print("\033[2J") // Clear the screen
	fmt.Print("\033[H")  // Move cursor to top-left corner

	args := []string{
		"aws",
		"ecs", "execute-command",
		"--cluster", cluster,
		"--task", task,
		"--container", container,
		"--interactive",
		"--command", command,
	}

	err := syscall.Exec("/usr/local/bin/aws", args, os.Environ())
	if err != nil {
		return fmt.Errorf("failed to execute command in container: %v", err)
	}

	return nil
}

func GetTaskDetails(ctx context.Context, ecsClient ECSClientAPI, cluster, taskArn string) ([]string, error) {
	input := &ecs.DescribeTasksInput{
		Cluster: &cluster,
		Tasks:   []string{taskArn},
	}

	output, err := ecsClient.DescribeTasks(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error describing task %s: %v", taskArn, err)
	}

	if len(output.Tasks) == 0 {
		return nil, fmt.Errorf("no task details found for task %s", taskArn)
	}

	var containerNames []string
	for _, container := range output.Tasks[0].Containers {
		containerNames = append(containerNames, *container.Name)
	}

	return containerNames, nil
}

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

// Service Updates Polling
// -----------------------

func PollServiceUpdates(ctx context.Context, ecsClient ECSClientAPI, services []pkg.ServiceDetails, updateInterval time.Duration) chan []pkg.ServiceDetails {
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
				updatedServices := make([]pkg.ServiceDetails, len(services))
				for i, service := range services {
					details, err := GetServiceDetails(ctx, ecsClient, service.ServiceName, service.Cluster)
					if err != nil {
						// Log the error, but continue with other services
						continue
					}
					updatedServices[i] = details
				}
				updates <- updatedServices
			}
		}
	}()

	return updates
}
