package aws

import (
	"context"
	"testing"

	"github.com/alexalbu001/bw-cli/pkg"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockECSClient is a mock of the ECS client
type MockECSClient struct {
	mock.Mock
}

func (m *MockECSClient) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ecs.ListClustersOutput), args.Error(1)
}

func (m *MockECSClient) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ecs.ListServicesOutput), args.Error(1)
}

func (m *MockECSClient) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ecs.DescribeServicesOutput), args.Error(1)
}

func (m *MockECSClient) UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ecs.UpdateServiceOutput), args.Error(1)
}

func (m *MockECSClient) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ecs.DescribeTasksOutput), args.Error(1)
}

func (m *MockECSClient) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ecs.ListTasksOutput), args.Error(1)
}

func TestGetAllServiceDetails(t *testing.T) {
	mockClient := new(MockECSClient)
	ctx := context.Background()

	// Mock ListClusters
	mockClient.On("ListClusters", ctx, mock.AnythingOfType("*ecs.ListClustersInput"), mock.Anything).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{"cluster1", "cluster2"},
	}, nil)

	// Mock ListServices for each cluster
	mockClient.On("ListServices", ctx, &ecs.ListServicesInput{Cluster: aws.String("cluster1")}, mock.Anything).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{"service1", "service2"},
	}, nil)
	mockClient.On("ListServices", ctx, &ecs.ListServicesInput{Cluster: aws.String("cluster2")}, mock.Anything).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{"service3", "service4"},
	}, nil)

	// Mock DescribeServices for each service in cluster1
	mockClient.On("DescribeServices", ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String("cluster1"),
		Services: []string{"service1", "service2"},
	}, mock.Anything).Return(&ecs.DescribeServicesOutput{
		Services: []types.Service{
			{
				ServiceName:  aws.String("service1"),
				RunningCount: 2,
				DesiredCount: 2,
				Status:       aws.String("ACTIVE"),
			},
			{
				ServiceName:  aws.String("service2"),
				RunningCount: 1,
				DesiredCount: 3,
				Status:       aws.String("DRAINING"),
			},
		},
	}, nil)

	// Mock DescribeServices for each service in cluster2
	mockClient.On("DescribeServices", ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String("cluster2"),
		Services: []string{"service3", "service4"},
	}, mock.Anything).Return(&ecs.DescribeServicesOutput{
		Services: []types.Service{
			{
				ServiceName:  aws.String("service3"),
				RunningCount: 3,
				DesiredCount: 3,
				Status:       aws.String("ACTIVE"),
			},
			{
				ServiceName:  aws.String("service4"),
				RunningCount: 0,
				DesiredCount: 2,
				Status:       aws.String("INACTIVE"),
			},
		},
	}, nil)

	services, err := GetAllServiceDetails(ctx, mockClient)

	assert.NoError(t, err)
	assert.Len(t, services, 4) // 2 clusters * 2 services each

	expectedServices := []pkg.ServiceDetails{
		{ServiceName: "service1", RunningCount: 2, DesiredCount: 2, Status: "ACTIVE", Cluster: "cluster1"},
		{ServiceName: "service2", RunningCount: 1, DesiredCount: 3, Status: "DRAINING", Cluster: "cluster1"},
		{ServiceName: "service3", RunningCount: 3, DesiredCount: 3, Status: "ACTIVE", Cluster: "cluster2"},
		{ServiceName: "service4", RunningCount: 0, DesiredCount: 2, Status: "INACTIVE", Cluster: "cluster2"},
	}

	assert.ElementsMatch(t, expectedServices, services)
	mockClient.AssertExpectations(t)
}

func TestUpdateServiceDesiredCount(t *testing.T) {
	mockClient := new(MockECSClient)
	ctx := context.Background()

	serviceName := "test-service"
	cluster := "test-cluster"
	initialDesiredCount := int32(2)
	newDesiredCount := int64(3)

	// Mock the update service call
	mockClient.On("UpdateService", ctx, mock.MatchedBy(func(input *ecs.UpdateServiceInput) bool {
		return *input.Cluster == cluster &&
			*input.Service == serviceName &&
			*input.DesiredCount == int32(newDesiredCount)
	}), mock.Anything).Return(&ecs.UpdateServiceOutput{
		Service: &types.Service{
			ServiceName:  aws.String(serviceName),
			DesiredCount: int32(newDesiredCount),
			RunningCount: initialDesiredCount,
		},
	}, nil).Once()

	err := UpdateServiceDesiredCount(ctx, mockClient, serviceName, cluster, newDesiredCount)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)

	mockClient.On("DescribeServices", ctx, mock.MatchedBy(func(input *ecs.DescribeServicesInput) bool {
		return *input.Cluster == cluster &&
			len(input.Services) == 1 &&
			input.Services[0] == serviceName
	}), mock.Anything).Return(&ecs.DescribeServicesOutput{
		Services: []types.Service{
			{
				ServiceName:  aws.String(serviceName),
				DesiredCount: int32(newDesiredCount),
				RunningCount: initialDesiredCount,
				Status:       aws.String("ACTIVE"),
			},
		},
	}, nil).Once()

	service, err := GetServiceDetails(ctx, mockClient, serviceName, cluster)
	assert.NoError(t, err)
	assert.Equal(t, newDesiredCount, service.DesiredCount)
	assert.Equal(t, int64(initialDesiredCount), service.RunningCount) // Running count should still be 2

	mockClient.AssertExpectations(t)
}

func TestGetServiceDetails(t *testing.T) {
	mockClient := new(MockECSClient)
	ctx := context.Background()

	serviceName := "test-service"
	cluster := "test-cluster"

	mockClient.On("DescribeServices", ctx, mock.AnythingOfType("*ecs.DescribeServicesInput"), mock.Anything).Return(&ecs.DescribeServicesOutput{
		Services: []types.Service{
			{
				ServiceName:  aws.String(serviceName),
				RunningCount: 2,
				DesiredCount: 2,
				Status:       aws.String("ACTIVE"),
			},
		},
	}, nil)

	service, err := GetServiceDetails(ctx, mockClient, serviceName, cluster)

	assert.NoError(t, err)
	assert.Equal(t, serviceName, service.ServiceName)
	assert.Equal(t, int64(2), service.RunningCount)
	assert.Equal(t, int64(2), service.DesiredCount)
	mockClient.AssertExpectations(t)
}
