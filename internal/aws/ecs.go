package aws

import (
	"aalbu/bw-cli/pkg"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
)

const maxDescribeServicesBatchSize = 10

// GetAllServiceDetails fetches services with running and desired count details from all clusters in parallel.
func GetAllServiceDetails(env string) ([]pkg.ServiceDetails, error) {
	clusters, err := listClusters(env)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	serviceCh := make(chan []pkg.ServiceDetails, len(clusters))

	for _, cluster := range clusters {
		wg.Add(1)
		go func(cluster string) {
			defer wg.Done()
			services, err := describeServicesInBatches(env, cluster)
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
func listClusters(env string) ([]string, error) {
	cmd := exec.Command("aws-vault", "exec", env, "--", "aws", "ecs", "list-clusters")
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
func describeServicesInBatches(env string, cluster string) ([]pkg.ServiceDetails, error) {
	serviceArns, err := listServices(env, cluster)
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
		cmd := exec.Command("aws-vault", append([]string{"exec", env, "--", "aws", "ecs", "describe-services", "--cluster", cluster, "--services"}, batch...)...)
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
func listServices(env, cluster string) ([]string, error) {
	cmd := exec.Command("aws-vault", "exec", env, "--", "aws", "ecs", "list-services", "--cluster", cluster)
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
func UpdateServiceDesiredCount(env, serviceName, cluster string, desiredCount int64) error {
	cmd := exec.Command("aws-vault", "exec", env, "--", "aws", "ecs", "update-service",
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
func RestartService(env, serviceName, cluster string) error {
	cmd := exec.Command("aws-vault", "exec", env, "--", "aws", "ecs", "update-service",
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
func GetServiceDeploymentStatus(env, serviceName, cluster string) (string, error) {
	cmd := exec.Command("aws-vault", "exec", env, "--", "aws", "ecs", "describe-services",
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
