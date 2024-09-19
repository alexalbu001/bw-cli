package main

import (
	"log"

	"aalbu/bw-cli/internal/aws"
	"aalbu/bw-cli/internal/ui"

	"github.com/rivo/tview"
)

func main() {

	// Fetch all service details (running and desired container counts)
	services, err := aws.GetAllServiceDetails()
	if err != nil {
		log.Fatalf("Error fetching services: %v", err)
	}

	// Create a new tview application instance
	app := tview.NewApplication()

	// Display the fetched services in the terminal UI, passing the app and env variable
	ui.DisplayServices(app, services)

	// Run the application
	if err := app.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}
