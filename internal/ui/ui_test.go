package ui

import (
	"context"
	"testing"

	"github.com/alexalbu001/bw-cli/pkg"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockECSClient is a mock of the ECS client
type MockECSClient struct {
	mock.Mock
}

// Implement the necessary methods of ECSClientAPI interface for MockECSClient

// MockApplication is a mock of the tview.Application
type MockApplication struct {
	mock.Mock
}

func (m *MockApplication) SetRoot(root tview.Primitive, fullscreen bool) *tview.Application {
	args := m.Called(root, fullscreen)
	return args.Get(0).(*tview.Application)
}

func (m *MockApplication) SetFocus(p tview.Primitive) *tview.Application {
	args := m.Called(p)
	return args.Get(0).(*tview.Application)
}

func TestNewServiceUI(t *testing.T) {
	app := &MockApplication{}
	ctx := context.Background()
	mockClient := &MockECSClient{}
	initialServices := []pkg.ServiceDetails{
		{ServiceName: "service1", RunningCount: 1, DesiredCount: 2, Status: "ACTIVE"},
		{ServiceName: "service2", RunningCount: 2, DesiredCount: 2, Status: "ACTIVE"},
	}

	serviceUI := NewServiceUI(app, ctx, mockClient, initialServices)

	assert.NotNil(t, serviceUI)
	assert.Equal(t, app, serviceUI.app)
	assert.Equal(t, ctx, serviceUI.ctx)
	assert.Equal(t, mockClient, serviceUI.ecsClient)
	assert.Equal(t, initialServices, serviceUI.currentServices)
	assert.Equal(t, initialServices, serviceUI.filteredServices)
	assert.NotNil(t, serviceUI.list)
	assert.NotNil(t, serviceUI.searchInput)
	assert.NotNil(t, serviceUI.layout)
}

func TestServiceUI_updateList(t *testing.T) {
	app := &MockApplication{}
	ctx := context.Background()
	mockClient := &MockECSClient{}
	initialServices := []pkg.ServiceDetails{
		{ServiceName: "service1", RunningCount: 1, DesiredCount: 2, Status: "ACTIVE"},
		{ServiceName: "service2", RunningCount: 2, DesiredCount: 2, Status: "ACTIVE"},
	}

	serviceUI := NewServiceUI(app, ctx, mockClient, initialServices)
	serviceUI.updateList()

	assert.Equal(t, 2, serviceUI.list.GetItemCount())
}

func TestServiceUI_filterServices(t *testing.T) {
	app := &MockApplication{}
	ctx := context.Background()
	mockClient := &MockECSClient{}
	initialServices := []pkg.ServiceDetails{
		{ServiceName: "service1", RunningCount: 1, DesiredCount: 2, Status: "ACTIVE"},
		{ServiceName: "service2", RunningCount: 2, DesiredCount: 2, Status: "ACTIVE"},
	}

	serviceUI := NewServiceUI(app, ctx, mockClient, initialServices)

	// Test filtering
	serviceUI.filterServices("service1")
	assert.Len(t, serviceUI.filteredServices, 1)
	assert.Equal(t, "service1", serviceUI.filteredServices[0].ServiceName)

	// Test no filter
	serviceUI.filterServices("")
	assert.Len(t, serviceUI.filteredServices, 2)
}

func TestServiceUI_setupSearchInput(t *testing.T) {
	app := &MockApplication{}
	ctx := context.Background()
	mockClient := &MockECSClient{}
	initialServices := []pkg.ServiceDetails{
		{ServiceName: "service1", RunningCount: 1, DesiredCount: 2, Status: "ACTIVE"},
		{ServiceName: "service2", RunningCount: 2, DesiredCount: 2, Status: "ACTIVE"},
	}

	serviceUI := NewServiceUI(app, ctx, mockClient, initialServices)
	serviceUI.setupSearchInput()

	// Test ESC key behavior
	event := tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)
	result := serviceUI.searchInput.InputHandler()(event, func(p tview.Primitive) {})
	assert.Nil(t, result)
	assert.Equal(t, "", serviceUI.searchInput.GetText())

	// Test Enter key behavior
	app.On("SetFocus", serviceUI.list).Return(app)
	event = tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	result = serviceUI.searchInput.InputHandler()(event, func(p tview.Primitive) {})
	assert.Nil(t, result)
	app.AssertExpectations(t)
}

func TestDisplayServices(t *testing.T) {
	app := &MockApplication{}
	ctx := context.Background()
	mockClient := &MockECSClient{}
	initialServices := []pkg.ServiceDetails{
		{ServiceName: "service1", RunningCount: 1, DesiredCount: 2, Status: "ACTIVE"},
		{ServiceName: "service2", RunningCount: 2, DesiredCount: 2, Status: "ACTIVE"},
	}

	app.On("SetRoot", mock.Anything, true).Return(app)
	app.On("SetFocus", mock.Anything).Return(app)

	DisplayServices(app, ctx, mockClient, initialServices)

	app.AssertExpectations(t)
}
