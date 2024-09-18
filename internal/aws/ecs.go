package aws

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"

	"aalbu/bw-cli/pkg"
)

const maxDescribeServicesBatchSize = 10

// GetAllServiceDetails fetches services with running and desired count details from all clusters in parallel
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

// listClusters fetches all ECS clusters
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

// describeServicesInBatches describes services for a given cluster in batches (max 10 services per batch)
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
		baseArgs := []string{"exec", env, "--", "aws", "ecs", "describe-services", "--cluster", cluster, "--services"}
		args := append(baseArgs, batch...)

		cmd := exec.Command("aws-vault", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Error executing 'describe-services' for batch in cluster %s: %v\n", cluster, err)
			fmt.Printf("Command output: %s\n", string(output))
			continue
		}

		var response struct {
			Services []pkg.ServiceDetails `json:"services"`
		}
		if err := json.Unmarshal(output, &response); err != nil {
			return nil, err
		}

		// Populate the Cluster field for each service
		for i := range response.Services {
			response.Services[i].Cluster = cluster
		}

		services = append(services, response.Services...)
	}

	return services, nil
}

// listServices fetches the ARNs of all services for a given ECS cluster and environment
func listServices(env string, cluster string) ([]string, error) {
	cmd := exec.Command("aws-vault", "exec", env, "--", "aws", "ecs", "list-services", "--cluster", cluster)
	output, err := cmd.CombinedOutput()

	if err != nil {
		fmt.Printf("Error executing command 'list-services' for cluster %s: %v\n", cluster, err)
		fmt.Printf("Command output: %s\n", string(output))
		return nil, err
	}

	var services pkg.ServiceOutput
	if err := json.Unmarshal(output, &services); err != nil {
		return nil, fmt.Errorf("error parsing JSON output: %v", err)
	}

	return services.ServiceArns, nil
}

// UpdateServiceDesiredCount updates the desired count for a given ECS service
func UpdateServiceDesiredCount(env, serviceName, cluster string, desiredCount int64) error {
	cmd := exec.Command("aws-vault", "exec", env, "--", "aws", "ecs", "update-service",
		"--cluster", cluster,
		"--service", serviceName,
		"--desired-count", fmt.Sprintf("%d", desiredCount))

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error updating service %s in cluster %s: %v\n", serviceName, cluster, err)
		fmt.Printf("Command output: %s\n", string(output))
		return err
	}

	fmt.Printf("Successfully updated %s in cluster %s to desired count %d\n", serviceName, cluster, desiredCount)
	return nil
}
