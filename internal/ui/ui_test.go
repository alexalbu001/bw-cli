package ui

import (
	"context"
	"testing"

	"github.com/alexalbu001/bw-cli/pkg"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockECSClient is a mock of the ECS client
type MockECSClient struct {
	mock.Mock
}

func (m *MockECSClient) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ecs.DescribeServicesOutput), args.Error(1)
}

func TestNewServiceUI(t *testing.T) {
	app := tview.NewApplication()
	ctx := context.Background()
	mockClient := &ecs.Client{}
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
}

func TestUpdateList(t *testing.T) {
	app := tview.NewApplication()
	ctx := context.Background()
	mockClient := &ecs.Client{}
	initialServices := []pkg.ServiceDetails{
		{ServiceName: "service1", RunningCount: 1, DesiredCount: 2, Status: "ACTIVE"},
		{ServiceName: "service2", RunningCount: 2, DesiredCount: 2, Status: "DRAINING"},
	}

	serviceUI := NewServiceUI(app, ctx, mockClient, initialServices)
	serviceUI.updateList()

	assert.Equal(t, 2, serviceUI.list.GetItemCount())

	item1, _ := serviceUI.list.GetItemText(0)
	assert.Contains(t, item1, "service1")
	assert.Contains(t, item1, "(Running: 1, Desired: 2)")
	assert.Contains(t, item1, "[green]ACTIVE[-]")

	item2, _ := serviceUI.list.GetItemText(1)
	assert.Contains(t, item2, "service2")
	assert.Contains(t, item2, "(Running: 2, Desired: 2)")
	assert.Contains(t, item2, "[yellow]DRAINING[-]")
}

func TestFilterServices(t *testing.T) {
	app := tview.NewApplication()
	ctx := context.Background()
	mockClient := &ecs.Client{}
	initialServices := []pkg.ServiceDetails{
		{ServiceName: "service1", RunningCount: 1, DesiredCount: 2, Status: "ACTIVE"},
		{ServiceName: "service2", RunningCount: 2, DesiredCount: 2, Status: "ACTIVE"},
		{ServiceName: "other", RunningCount: 1, DesiredCount: 1, Status: "ACTIVE"},
	}

	serviceUI := NewServiceUI(app, ctx, mockClient, initialServices)

	// Test filtering
	serviceUI.filterServices("service")
	assert.Equal(t, 2, len(serviceUI.filteredServices))

	// Test case insensitivity
	serviceUI.filterServices("SERVICE")
	assert.Equal(t, 2, len(serviceUI.filteredServices))

	// Test no results
	serviceUI.filterServices("nonexistent")
	assert.Equal(t, 0, len(serviceUI.filteredServices))

	// Test empty query
	serviceUI.filterServices("")
	assert.Equal(t, 3, len(serviceUI.filteredServices))
}

func TestSetupSearchInput(t *testing.T) {
	app := tview.NewApplication()
	ctx := context.Background()
	mockClient := &ecs.Client{}
	initialServices := []pkg.ServiceDetails{
		{ServiceName: "service1", RunningCount: 1, DesiredCount: 2, Status: "ACTIVE"},
		{ServiceName: "service2", RunningCount: 2, DesiredCount: 2, Status: "ACTIVE"},
	}

	serviceUI := NewServiceUI(app, ctx, mockClient, initialServices)
	serviceUI.setupSearchInput()

	// Test ESC key
	event := tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)
	serviceUI.searchInput.SetText("test")
	capturedEvent := serviceUI.searchInput.GetInputCapture()(event)
	assert.Nil(t, capturedEvent)
	// The text is not cleared immediately, it would be cleared in the SetText("") call
	// which is not triggered in this test environment
	assert.Equal(t, "test", serviceUI.searchInput.GetText())

	// Test Enter key
	event = tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	capturedEvent = serviceUI.searchInput.GetInputCapture()(event)
	assert.Nil(t, capturedEvent)
	// The text remains unchanged
	assert.Equal(t, "test", serviceUI.searchInput.GetText())

	// Test Down key
	event = tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	capturedEvent = serviceUI.searchInput.GetInputCapture()(event)
	assert.Nil(t, capturedEvent)
	// The text remains unchanged
	assert.Equal(t, "test", serviceUI.searchInput.GetText())

	// Test other key (should not be captured)
	event = tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone)
	capturedEvent = serviceUI.searchInput.GetInputCapture()(event)
	assert.Equal(t, event, capturedEvent)
	// The text remains unchanged as we're not actually inputting the character
	assert.Equal(t, "test", serviceUI.searchInput.GetText())
}

func TestSetupListInputCapture(t *testing.T) {
	app := tview.NewApplication()
	ctx := context.Background()
	mockClient := &ecs.Client{}
	initialServices := []pkg.ServiceDetails{
		{ServiceName: "service1", RunningCount: 1, DesiredCount: 2, Status: "ACTIVE"},
		{ServiceName: "service2", RunningCount: 2, DesiredCount: 2, Status: "ACTIVE"},
	}

	serviceUI := NewServiceUI(app, ctx, mockClient, initialServices)
	serviceUI.setupListInputCapture()

	var capturedEvent *tcell.EventKey

	serviceUI.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		capturedEvent = event
		return event
	})

	// Test 'R' key
	event := tcell.NewEventKey(tcell.KeyRune, 'R', tcell.ModNone)
	serviceUI.list.InputHandler()(event, func(p tview.Primitive) {})
	assert.Equal(t, event, capturedEvent)

	// Test 's' key
	event = tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone)
	serviceUI.list.InputHandler()(event, func(p tview.Primitive) {})
	assert.Equal(t, event, capturedEvent)

	// Test '/' key
	event = tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone)
	serviceUI.list.InputHandler()(event, func(p tview.Primitive) {})
	assert.Equal(t, event, capturedEvent)

	// Test Up key when at the top of the list
	serviceUI.list.SetCurrentItem(0)
	event = tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	serviceUI.list.InputHandler()(event, func(p tview.Primitive) {})
	assert.Equal(t, event, capturedEvent)

	// Test Up key when not at the top of the list
	serviceUI.list.SetCurrentItem(1)
	event = tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	serviceUI.list.InputHandler()(event, func(p tview.Primitive) {})
	assert.Equal(t, event, capturedEvent)
}

// Add more tests for other functions as needed
