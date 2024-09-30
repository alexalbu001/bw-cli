package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexalbu001/bw-cli/internal/aws"
	"github.com/alexalbu001/bw-cli/pkg"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ServiceUI struct {
	app              *tview.Application
	ctx              context.Context
	ecsClient        *ecs.Client
	list             *tview.List
	searchInput      *tview.InputField
	currentServices  []pkg.ServiceDetails
	filteredServices []pkg.ServiceDetails
	layout           *tview.Flex
	header           *tview.TextView
	logo             *tview.TextView
}

func NewServiceUI(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, initialServices []pkg.ServiceDetails) *ServiceUI {
	s := &ServiceUI{
		app:              app,
		ctx:              ctx,
		ecsClient:        ecsClient,
		list:             tview.NewList(),
		searchInput:      tview.NewInputField().SetLabel("/ "),
		currentServices:  initialServices,
		filteredServices: initialServices,
		header:           tview.NewTextView().SetTextAlign(tview.AlignLeft),
		logo:             tview.NewTextView().SetTextAlign(tview.AlignRight),
	}
	s.layout = s.createLayout()
	return s
}

func (s *ServiceUI) updateList() {
	s.list.Clear()
	for i, service := range s.filteredServices {
		index := i
		status := service.Status
		statusColor := "[white]"
		switch strings.ToLower(status) {
		case "active":
			statusColor = "[green]"
		case "draining":
			statusColor = "[yellow]"
		case "inactive":
			statusColor = "[red]"
		}
		s.list.AddItem(
			fmt.Sprintf("%s (Running: %d, Desired: %d) - Status: %s%s[-]",
				service.ServiceName, service.RunningCount, service.DesiredCount, statusColor, status),
			"", 0, func() {
				showServiceOptions(s.app, s.ctx, s.ecsClient, s.filteredServices[index], s.filteredServices, s.layout)
			})
	}
	s.updateHeader()
}

func (s *ServiceUI) updateHeader() {
	s.header.Clear()
	fmt.Fprintf(s.header, "Total Services: %d", len(s.currentServices))
}

func (s *ServiceUI) filterServices(query string) {
	if query == "" {
		s.filteredServices = s.currentServices
	} else {
		s.filteredServices = []pkg.ServiceDetails{}
		for _, service := range s.currentServices {
			if strings.Contains(strings.ToLower(service.ServiceName), strings.ToLower(query)) {
				s.filteredServices = append(s.filteredServices, service)
			}
		}
	}
	s.updateList()
}

func (s *ServiceUI) setupSearchInput() {
	s.searchInput.
		SetChangedFunc(func(text string) {
			s.filterServices(text)
		}).
		SetFieldBackgroundColor(tcell.GetColor("#000000"))

	s.searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			s.searchInput.SetText("")
			s.filterServices("")
			s.app.SetFocus(s.list)
			return nil
		case tcell.KeyEnter, tcell.KeyDown:
			if s.list.GetItemCount() > 0 {
				s.app.SetFocus(s.list)
			}
			return nil
		}
		return event
	})
}

func (s *ServiceUI) setupListInputCapture() {
	s.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'R': // Restart all services
				showRestartAllServicesPrompt(s.app, s.ctx, s.ecsClient, s.currentServices, s.layout)
			case 's': // Connect to a container
				if s.list.GetItemCount() > 0 {
					currentService := s.filteredServices[s.list.GetCurrentItem()]
					showContainerExecPrompt(s.app, s.ctx, s.ecsClient, currentService)
				}
			case '/': // Activate search
				s.app.SetFocus(s.searchInput)
				return nil
			}
		case tcell.KeyUp:
			if s.list.GetCurrentItem() == 0 {
				s.app.SetFocus(s.searchInput)
				return nil
			}
		}
		return event
	})
}

func (s *ServiceUI) startPolling() {
	updateInterval := 10 * time.Second
	updates := aws.PollServiceUpdates(s.ctx, s.ecsClient, s.currentServices, updateInterval)

	go func() {
		for updatedServices := range updates {
			s.app.QueueUpdateDraw(func() {
				s.currentServices = updatedServices
				s.filterServices(s.searchInput.GetText())
			})
		}
	}()
}

func (s *ServiceUI) createLayout() *tview.Flex {
	legend := tview.NewTextView().
		SetText("[yellow]s[-] - Shell | [red]R[-] - Redeploy all containers | [#69359C]/[-] - Search").
		SetTextColor(tcell.ColorWhite).
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	// Create a frame for the list
	listFrame := tview.NewFrame(s.list).
		SetBorders(0, 0, 0, 0, 0, 0)

	// Set up the ASCII art logo
	s.logo.SetText(` 
    ____  _       __     ____ __    ____
   / __ )| |     / /    / __ / /   /  _/
  / __  | | /| / /    / / // /    / /  
 / /_/ /| |/ |/ /    / /_// /____/ /   
/_____/ |__/|__/     \_,_/_____/___/   
`).SetTextColor(tcell.ColorYellow)

	// Create the top bar with header and logo
	topBar := tview.NewFlex().
		AddItem(s.header, 0, 1, false).
		AddItem(s.logo, 0, 1, false)

	// Create the main layout
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(topBar, 6, 1, false).
		AddItem(s.searchInput, 1, 1, false).
		AddItem(listFrame, 0, 1, true).
		AddItem(legend, 1, 1, false)

	return mainFlex
}

func DisplayServices(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, initialServices []pkg.ServiceDetails) {
	serviceUI := NewServiceUI(app, ctx, ecsClient, initialServices)

	serviceUI.updateList()
	serviceUI.setupSearchInput()
	serviceUI.setupListInputCapture()
	serviceUI.startPolling()

	app.SetRoot(serviceUI.layout, true)
	app.SetFocus(serviceUI.list)
}

// showServiceOptions shows available options for a specific service.
func showServiceOptions(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, service pkg.ServiceDetails, services []pkg.ServiceDetails, layout *tview.Flex) {
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Service: %s\nChoose an action:", service.ServiceName)).
		AddButtons([]string{"Change Desired Count", "Restart Service", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonLabel {
			case "Change Desired Count":
				showDesiredCountPrompt(app, ctx, ecsClient, service, services, layout)
			case "Restart Service":
				restartService(app, ctx, ecsClient, service, layout)
			default:
				app.SetRoot(layout, true)
			}
		})

	app.SetRoot(modal, false)
}

// restartService redeploys only the selected service
func restartService(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, service pkg.ServiceDetails, layout *tview.Flex) {
	err := aws.RestartService(ctx, ecsClient, service.ServiceName, service.Cluster)
	if err != nil {
		showMessage(app, fmt.Sprintf("Failed to restart service: %v", err), layout)
	} else {
		showMessage(app, fmt.Sprintf("Service %s has been restarted.", service.ServiceName), layout)
	}
}

// showRestartAllServicesPrompt shows a confirmation prompt to restart all services.
func showRestartAllServicesPrompt(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, services []pkg.ServiceDetails, layout *tview.Flex) {
	modal := tview.NewModal().
		SetText("Are you sure you want to restart all services?").
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Yes" {
				go restartAllServices(app, ctx, ecsClient, services, layout)
			}
			app.SetRoot(layout, true)
		})

	app.SetRoot(modal, false)
}

// restartAllServices triggers redeploys of all services in the background
func restartAllServices(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, services []pkg.ServiceDetails, layout *tview.Flex) {
	var wg sync.WaitGroup
	failedServices := make(chan string, len(services))

	for _, service := range services {
		wg.Add(1)
		go func(s pkg.ServiceDetails) {
			defer wg.Done()
			if err := aws.RestartService(ctx, ecsClient, s.ServiceName, s.Cluster); err != nil {
				failedServices <- s.ServiceName
			}
		}(service)
	}

	wg.Wait()
	close(failedServices)

	failed := make([]string, 0, len(services))
	for s := range failedServices {
		failed = append(failed, s)
	}

	app.QueueUpdateDraw(func() {
		if len(failed) > 0 {
			showMessage(app, fmt.Sprintf("Failed to restart services: %v", failed), layout)
		} else {
			showMessage(app, "All services have been restarted successfully.", layout)
		}
	})
}

// showDesiredCountPrompt shows a prompt to change the desired count for the selected service
func showDesiredCountPrompt(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, service pkg.ServiceDetails, services []pkg.ServiceDetails, layout *tview.Flex) {
	inputField := tview.NewInputField().
		SetLabel(fmt.Sprintf("Change desired count for %s: ", service.ServiceName)).
		SetFieldWidth(5)

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			newDesiredCount, err := strconv.Atoi(inputField.GetText())
			if err != nil {
				showMessage(app, "Invalid input. Please enter a positive integer.", layout)
				return
			}

			err = aws.UpdateServiceDesiredCount(ctx, ecsClient, service.ServiceName, service.Cluster, int64(newDesiredCount))
			if err != nil {
				showMessage(app, fmt.Sprintf("Failed to update service: %v", err), layout)
				return
			}

			showMessage(app, fmt.Sprintf("Updated %s to desired count %d. The running count will update shortly.",
				service.ServiceName, newDesiredCount), layout)
		}
	})

	app.SetRoot(inputField, true)
}

// showMessage shows a modal with a message and an OK button that returns to the service list.
func showMessage(app *tview.Application, message string, previousView tview.Primitive) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(previousView, true)
		})

	app.SetRoot(modal, false)
}

// showContainerExecPrompt prompts for the container and command, then connects to the container using ECS Exec.
func showContainerExecPrompt(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, service pkg.ServiceDetails) {
	// Fetch the task associated with the service
	taskArn, err := aws.GetTaskArnForService(ctx, ecsClient, service.Cluster, service.ServiceName)
	if err != nil {
		showMessage(app, fmt.Sprintf("Failed to fetch task for service: %v", err), nil)
		return
	}

	// Fetch container names for the task
	containerNames, err := aws.GetTaskDetails(ctx, ecsClient, service.Cluster, taskArn)
	if err != nil {
		showMessage(app, fmt.Sprintf("Failed to fetch containers for task: %v", err), nil)
		return
	}

	// Present container selection modal
	showContainerSelection(app, ctx, ecsClient, service.Cluster, taskArn, containerNames)
}

// showContainerSelection presents a list of containers for the user to choose from.
func showContainerSelection(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, cluster, taskArn string, containerNames []string) {
	list := tview.NewList()
	for _, containerName := range containerNames {
		container := containerName // Capture the current containerName in the loop
		list.AddItem(containerName, "", 0, func() {
			// Show a confirmation modal
			modal := tview.NewModal().
				SetText(fmt.Sprintf("Connect to container %s?", container)).
				AddButtons([]string{"Connect", "Cancel"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					if buttonLabel == "Connect" {
						// Connect to the selected container
						command := "/bin/sh"
						err := aws.ExecCommandToContainer(cluster, taskArn, container, command)
						if err != nil {
							showMessage(app, fmt.Sprintf("Failed to connect to container: %v", err), list)
						} else {
							showMessage(app, fmt.Sprintf("Connecting to container %s...", container), list)
						}
					} else {
						// Return to the container selection list
						app.SetRoot(list, true)
					}
				})

			app.SetRoot(modal, true)
		})
	}

	list.SetDoneFunc(func() {
		// Return to the previous screen
		app.Stop()
	})

	app.SetRoot(list, true)
}
