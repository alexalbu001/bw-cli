package ui

import (
	"fmt"
	"strconv"

	"aalbu/bw-cli/internal/aws"
	"aalbu/bw-cli/pkg"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DisplayServices shows the services in a scrollable UI and allows updating the desired count
func DisplayServices(app *tview.Application, services []pkg.ServiceDetails, env string) {
	list := tview.NewList()

	// Add services to the list
	for _, service := range services {
		list.AddItem(fmt.Sprintf("%s (Running: %d, Desired: %d)", service.ServiceName, service.RunningCount, service.DesiredCount), "", 0, func() {})
	}

	// Set the selected service action to update the desired count
	list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selectedService := services[index]
		showDesiredCountPrompt(app, selectedService, env, services, list)
	})

	app.SetRoot(list, true)
}

// showDesiredCountPrompt shows a prompt to input the new desired count for a service
func showDesiredCountPrompt(app *tview.Application, service pkg.ServiceDetails, env string, services []pkg.ServiceDetails, list *tview.List) {
	inputField := tview.NewInputField().
		SetLabel(fmt.Sprintf("Change desired count for %s: ", service.ServiceName)).
		SetFieldWidth(5)

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			newDesiredCount, err := convertInputToInt(inputField.GetText())
			if err != nil {
				showMessage(app, "Invalid input. Please enter a positive integer.", list)
				return
			}

			err = updateServiceDesiredCount(app, service, env, newDesiredCount, list)
			if err != nil {
				showMessage(app, fmt.Sprintf("Failed to update service: %v", err), list)
			} else {
				showMessage(app, fmt.Sprintf("Updated %s to desired count %d", service.ServiceName, newDesiredCount), list)
			}
		}
	})

	app.SetRoot(inputField, true)
}

func convertInputToInt(input string) (int, error) {
	return strconv.Atoi(input)
}

func updateServiceDesiredCount(app *tview.Application, service pkg.ServiceDetails, env string, newDesiredCount int, list *tview.List) error {
	err := aws.UpdateServiceDesiredCount(env, service.ServiceName, service.Cluster, int64(newDesiredCount))
	if err != nil {
		return err
	}

	// Update the UI to reflect the new desired count
	service.DesiredCount = int64(newDesiredCount)
	list.SetItemText(list.GetCurrentItem(), fmt.Sprintf("%s (Running: %d, Desired: %d)", service.ServiceName, service.RunningCount, service.DesiredCount), "")
	return nil
}

// showMessage shows a modal with a message and an OK button that returns to the previous screen
func showMessage(app *tview.Application, message string, previousView tview.Primitive) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			// Return to the previous view when the user presses OK
			app.SetRoot(previousView, true)
		})

	app.SetRoot(modal, false)
}
