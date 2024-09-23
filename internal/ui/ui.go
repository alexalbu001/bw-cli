package ui

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/alexalbu001/bw-cli/pkg"

	"github.com/alexalbu001/bw-cli/internal/aws"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DisplayServices shows the services and their deployment status, along with a legend.
func DisplayServices(app *tview.Application, services []pkg.ServiceDetails) {
	// Create the list of services
	list := tview.NewList()
	for i, service := range services {
		index := i
		list.AddItem(
			fmt.Sprintf("%s (Running: %d, Desired: %d) - Status: %s",
				service.ServiceName, service.RunningCount, service.DesiredCount, service.Status),
			"", 0, func() {
				showServiceOptions(app, services[index], services, list)
			})
	}

	// Capture key inputs for actions
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'R': // Restart all services
			showRestartAllServicesPrompt(app, services, list)
		case 's': // Connect to a container
			currentService := services[list.GetCurrentItem()]
			showContainerExecPrompt(app, currentService)
		}
		return event
	})

	// Create a legend text view
	legend := tview.NewTextView().
		SetText("[yellow]s[-] - Shell | [red]R[-] - Redeploy all containers").
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	// Create a flex layout to hold the service list and the legend
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).   // The list will take most of the screen space
		AddItem(legend, 1, 1, false) // The legend will be at the bottom of the screen

	// Set the root layout and run the app
	app.SetRoot(flex, true)
}

// startPollingDeploymentStatus polls for updates on service status.
func startPollingDeploymentStatus(app *tview.Application, list *tview.List, services []pkg.ServiceDetails) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		updatedServices, err := fetchUpdatedServices(services)
		if err != nil {
			fmt.Println("Error fetching updated services:", err)
			continue
		}

		app.QueueUpdateDraw(func() {
			list.Clear()
			for i, service := range updatedServices {
				index := i
				list.AddItem(
					fmt.Sprintf("%s (Running: %d, Desired: %d) - Status: %s",
						service.ServiceName, service.RunningCount, service.DesiredCount, service.Status),
					"", 0, func() {
						showServiceOptions(app, updatedServices[index], updatedServices, list)
					})
			}
		})
	}
}

// fetchUpdatedServices fetches the updated deployment status for each service.
func fetchUpdatedServices(services []pkg.ServiceDetails) ([]pkg.ServiceDetails, error) {
	for i, service := range services {
		status, err := aws.GetServiceDeploymentStatus(service.ServiceName, service.Cluster)
		if err != nil {
			return nil, err
		}
		services[i].Status = status
	}
	return services, nil
}

// showServiceOptions shows available options for a specific service.
func showServiceOptions(app *tview.Application, service pkg.ServiceDetails, services []pkg.ServiceDetails, list *tview.List) {
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Service: %s\nChoose an action:", service.ServiceName)).
		AddButtons([]string{"Change Desired Count", "Restart Service", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonLabel {
			case "Change Desired Count":
				showDesiredCountPrompt(app, service, services, list)
			case "Restart Service":
				restartService(app, service, list)
			default:
				app.SetRoot(list, true)
			}
		})

	app.SetRoot(modal, false)
}

// restartService redeploys only the selected service
func restartService(app *tview.Application, service pkg.ServiceDetails, list *tview.List) {
	err := aws.RestartService(service.ServiceName, service.Cluster)
	if err != nil {
		showMessage(app, fmt.Sprintf("Failed to restart service: %v", err), list)
	} else {
		showMessage(app, fmt.Sprintf("Service %s has been restarted.", service.ServiceName), list)
	}
}

// showRestartAllServicesPrompt shows a confirmation prompt to restart all services.
func showRestartAllServicesPrompt(app *tview.Application, services []pkg.ServiceDetails, list *tview.List) {
	modal := tview.NewModal().
		SetText("Are you sure you want to restart all services?").
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Yes" {
				go restartAllServices(app, services, list)
			}
			app.SetRoot(list, true)
		})

	app.SetRoot(modal, false)
}

// restartAllServices triggers redeploys of all services in the background
func restartAllServices(app *tview.Application, services []pkg.ServiceDetails, list *tview.List) {
	var wg sync.WaitGroup
	failedServices := make(chan string, len(services))

	for _, service := range services {
		wg.Add(1)
		go func(s pkg.ServiceDetails) {
			defer wg.Done()
			if err := aws.RestartService(s.ServiceName, s.Cluster); err != nil {
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
			showMessage(app, fmt.Sprintf("Failed to restart services: %v", failed), list)
		} else {
			showMessage(app, "All services have been restarted successfully.", list)
		}
	})
}

// showDesiredCountPrompt shows a prompt to change the desired count for the selected service
func showDesiredCountPrompt(app *tview.Application, service pkg.ServiceDetails, services []pkg.ServiceDetails, list *tview.List) {
	inputField := tview.NewInputField().
		SetLabel(fmt.Sprintf("Change desired count for %s: ", service.ServiceName)).
		SetFieldWidth(5)

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			newDesiredCount, err := strconv.Atoi(inputField.GetText())
			if err != nil {
				showMessage(app, "Invalid input. Please enter a positive integer.", list)
				return
			}

			// Only update the desired count of the selected service
			err = aws.UpdateServiceDesiredCount(service.ServiceName, service.Cluster, int64(newDesiredCount))
			if err != nil {
				showMessage(app, fmt.Sprintf("Failed to update service: %v", err), list)
			} else {
				service.DesiredCount = int64(newDesiredCount)
				list.SetItemText(list.GetCurrentItem(), fmt.Sprintf("%s (Running: %d, Desired: %d)", service.ServiceName, service.RunningCount, service.DesiredCount), "")
				showMessage(app, fmt.Sprintf("Updated %s to desired count %d", service.ServiceName, newDesiredCount), list)
			}
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
func showContainerExecPrompt(app *tview.Application, service pkg.ServiceDetails) {
	// Fetch the task associated with the service
	taskArn, err := aws.GetTaskArnForService(service.Cluster, service.ServiceName)
	if err != nil {
		showMessage(app, fmt.Sprintf("Failed to fetch task for service: %v", err), nil)
		return
	}

	// Fetch container names for the task
	containerNames, err := aws.GetTaskDetails(service.Cluster, taskArn)
	if err != nil {
		showMessage(app, fmt.Sprintf("Failed to fetch containers for task: %v", err), nil)
		return
	}

	// Present container selection modal
	showContainerSelection(app, service.Cluster, taskArn, containerNames)
}

// showContainerSelection presents a list of containers for the user to choose from.
func showContainerSelection(app *tview.Application, cluster, taskArn string, containerNames []string) {
	list := tview.NewList()
	for _, containerName := range containerNames {
		container := containerName // Capture the current containerName in the loop
		list.AddItem(containerName, "", 0, func() {
			// Connect to the selected container
			command := "/bin/sh" // Default command to open a shell
			err := aws.ExecCommandToContainer(cluster, taskArn, container, command)
			if err != nil {
				showMessage(app, fmt.Sprintf("Failed to connect to container: %v", err), nil)
			} else {
				showMessage(app, fmt.Sprintf("Connected to container %s successfully!", container), nil)
			}
		})
	}

	list.SetDoneFunc(func() {
		// Return to the previous screen if needed
		app.Stop()
	})

	app.SetRoot(list, true)
}
