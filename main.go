package main

import (
	"log"

	"github.com/alexalbu001/bw-cli/internal/aws"
	"github.com/alexalbu001/bw-cli/internal/ui"

	"github.com/rivo/tview"
)

func main() {

	err := aws.GetCallerIdentity()
	if err != nil {
		log.Fatalf("Error fetching AWS identity: %v", err)
	}

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
