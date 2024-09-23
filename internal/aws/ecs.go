package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/alexalbu001/bw-cli/pkg"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const maxDescribeServicesBatchSize = 10

// GetAllServiceDetails fetches services with running and desired count details from all clusters in parallel.
func GetAllServiceDetails() ([]pkg.ServiceDetails, error) {
	clusters, err := listClusters()
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	serviceCh := make(chan []pkg.ServiceDetails, len(clusters))

	for _, cluster := range clusters {
		wg.Add(1)
		go func(cluster string) {
			defer wg.Done()
			services, err := describeServicesInBatches(cluster)
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

// listClusters fetches all ECS clusters.
func listClusters() ([]string, error) {
	cmd := exec.Command("aws", "ecs", "list-clusters")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error listing clusters: %v", err)
	}

	var clusters pkg.ClusterOutput
	if err := json.Unmarshal(output, &clusters); err != nil {
		return nil, fmt.Errorf("error parsing cluster response: %v", err)
	}

	return clusters.ClusterArns, nil
}

// describeServicesInBatches describes services for a given cluster in batches.
func describeServicesInBatches(cluster string) ([]pkg.ServiceDetails, error) {
	serviceArns, err := listServices(cluster)
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
		cmd := exec.Command("aws", append([]string{"ecs", "describe-services", "--cluster", cluster, "--services"}, batch...)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Error describing services in cluster %s: %v\n", cluster, err)
			continue
		}

		var response struct {
			Services []pkg.ServiceDetails `json:"services"`
		}
		if err := json.Unmarshal(output, &response); err != nil {
			return nil, err
		}

		// Add cluster name to each service
		for i := range response.Services {
			response.Services[i].Cluster = cluster
		}

		services = append(services, response.Services...)
	}

	return services, nil
}

// listServices fetches the ARNs of all services for a given ECS cluster.
func listServices(cluster string) ([]string, error) {
	cmd := exec.Command("aws", "ecs", "list-services", "--cluster", cluster)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error listing services in cluster %s: %v\n", cluster, err)
		return nil, err
	}

	var services pkg.ServiceOutput
	if err := json.Unmarshal(output, &services); err != nil {
		return nil, fmt.Errorf("error parsing JSON output: %v", err)
	}

	return services.ServiceArns, nil
}

// UpdateServiceDesiredCount updates the desired count for a given ECS service.
func UpdateServiceDesiredCount(serviceName, cluster string, desiredCount int64) error {
	cmd := exec.Command("aws", "ecs", "update-service",
		"--cluster", cluster,
		"--service", serviceName,
		"--desired-count", fmt.Sprintf("%d", desiredCount))

	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update service %s in cluster %s: %v", serviceName, cluster, err)
	}
	return nil
}

// RestartService forces a redeploy of the ECS service by calling the update-service command.
func RestartService(serviceName, cluster string) error {
	cmd := exec.Command("aws", "ecs", "update-service",
		"--cluster", cluster,
		"--service", serviceName,
		"--force-new-deployment")

	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %v", serviceName, err)
	}

	return nil
}

// GetServiceDeploymentStatus fetches the deployment status of a specific ECS service.
func GetServiceDeploymentStatus(serviceName, cluster string) (string, error) {
	cmd := exec.Command("aws", "ecs", "describe-services",
		"--cluster", cluster,
		"--services", serviceName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error describing service %s: %v", serviceName, err)
	}

	var response struct {
		Services []struct {
			Deployments []struct {
				RolloutState string `json:"rolloutState"`
				RunningCount int64  `json:"runningCount"`
				DesiredCount int64  `json:"desiredCount"`
				Status       string `json:"status"`
			} `json:"deployments"`
		} `json:"services"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return "", err
	}

	if len(response.Services) == 0 || len(response.Services[0].Deployments) == 0 {
		return "Unknown", nil
	}

	// Extract deployment details
	deployment := response.Services[0].Deployments[0]
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
	return deployment.Status, nil
}

// ExecCommandToContainer executes a command inside the ECS container using ECS Exec.
func ExecCommandToContainer(cluster, task, container, command string) error {
	// Prepare the command arguments for ECS Exec
	args := []string{
		"ecs", "execute-command",
		"--cluster", cluster,
		"--task", task,
		"--container", container,
		"--interactive",
		"--command", command,
	}

	// Replace the current Go process with the AWS CLI command
	err := syscall.Exec("/usr/local/bin/aws", append([]string{"aws"}, args...), os.Environ()) // Use the correct path to aws
	if err != nil {
		return fmt.Errorf("failed to execute command in container: %v", err)
	}

	return nil
}

// GetTaskDetails fetches details for a running task, including the container names.
func GetTaskDetails(cluster, taskArn string) ([]string, error) {
	cmd := exec.Command("aws", "ecs", "describe-tasks",
		"--cluster", cluster,
		"--tasks", taskArn)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error describing task %s: %v", taskArn, err)
	}

	var taskResponse struct {
		Tasks []struct {
			Containers []struct {
				Name string `json:"name"`
			} `json:"containers"`
		} `json:"tasks"`
	}

	if err := json.Unmarshal(output, &taskResponse); err != nil {
		return nil, fmt.Errorf("error parsing task details: %v", err)
	}

	if len(taskResponse.Tasks) == 0 {
		return nil, fmt.Errorf("no task details found for task %s", taskArn)
	}

	// Extract container names
	var containerNames []string
	for _, container := range taskResponse.Tasks[0].Containers {
		containerNames = append(containerNames, container.Name)
	}

	return containerNames, nil
}

// GetTaskArnForService fetches the ARN of the running task for the specified service.
func GetTaskArnForService(cluster, serviceName string) (string, error) {
	cmd := exec.Command("aws", "ecs", "list-tasks",
		"--cluster", cluster,
		"--service-name", serviceName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error listing tasks for service %s: %v", serviceName, err)
	}

	var taskResponse struct {
		TaskArns []string `json:"taskArns"`
	}

	if err := json.Unmarshal(output, &taskResponse); err != nil {
		return "", fmt.Errorf("error parsing task list: %v", err)
	}

	if len(taskResponse.TaskArns) == 0 {
		return "", fmt.Errorf("no running tasks found for service %s", serviceName)
	}

	// Returning the first task ARN (assuming only one task is running)
	return taskResponse.TaskArns[0], nil
}

func GetCallerIdentity() error {
	// Load the default AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}

	// Create an STS client
	stsClient := sts.NewFromConfig(cfg)

	// Call the GetCallerIdentity API
	identityOutput, err := stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("failed to get caller identity: %v", err)
	}

	// Display the account ID and ARN
	fmt.Printf("AWS Account ID: %s\n", *identityOutput.Account)
	fmt.Printf("ARN: %s\n", *identityOutput.Arn)
	fmt.Printf("User ID: %s\n", *identityOutput.UserId)

	return nil
}
