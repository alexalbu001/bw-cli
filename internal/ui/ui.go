// File: internal/ui/ui.go

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexalbu001/bw-cli/internal/aws"
	"github.com/alexalbu001/bw-cli/pkg"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ServiceUI struct {
	app              *tview.Application
	ctx              context.Context
	ecsClient        *ecs.Client
	cwClient         *cloudwatch.Client
	list             *tview.List
	searchInput      *tview.InputField
	currentServices  []pkg.ServiceDetails
	filteredServices []pkg.ServiceDetails
	layout           *tview.Flex
	header           *tview.TextView
	logo             *tview.TextView
}

func NewServiceUI(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, cwClient *cloudwatch.Client, initialServices []pkg.ServiceDetails) *ServiceUI {
	s := &ServiceUI{
		app:              app,
		ctx:              ctx,
		ecsClient:        ecsClient,
		cwClient:         cwClient,
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
	for _, service := range s.filteredServices {
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
			fmt.Sprintf("%s (Running: %d, Desired: %d) - Status: %s%s[-] | CPU: %.2f%%, Mem: %.2f%%",
				service.ServiceName, service.RunningCount, service.DesiredCount, statusColor, status,
				service.CPUUtilization, service.MemoryUtilization),
			"", 0, nil)
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
	updates := aws.PollServiceUpdates(s.ctx, s.ecsClient, s.cwClient, updateInterval)

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
		SetText("[yellow]/[-] - Search | [red]R[-] - Redeploy all containers").
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

func DisplayServices(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, cwClient *cloudwatch.Client, initialServices []pkg.ServiceDetails) {
	serviceUI := NewServiceUI(app, ctx, ecsClient, cwClient, initialServices)

	serviceUI.updateList()
	serviceUI.setupSearchInput()
	serviceUI.setupListInputCapture()
	serviceUI.startPolling()

	app.SetRoot(serviceUI.layout, true)
	app.SetFocus(serviceUI.list)
}

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

func restartAllServices(app *tview.Application, ctx context.Context, ecsClient *ecs.Client, services []pkg.ServiceDetails, layout *tview.Flex) {
	failedServices := []string{}

	for _, service := range services {
		err := aws.RestartService(ctx, ecsClient, service.ServiceName, service.Cluster)
		if err != nil {
			failedServices = append(failedServices, service.ServiceName)
		}
	}

	app.QueueUpdateDraw(func() {
		if len(failedServices) > 0 {
			showMessage(app, fmt.Sprintf("Failed to restart services: %v", failedServices), layout)
		} else {
			showMessage(app, "All services have been restarted successfully.", layout)
		}
	})
}

func showMessage(app *tview.Application, message string, previousView tview.Primitive) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(previousView, true)
		})

	app.SetRoot(modal, false)
}
