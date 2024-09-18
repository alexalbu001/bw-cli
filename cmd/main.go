package main

import (
	"flag"
	"log"

	"aalbu/bw-cli/internal/aws"
	"aalbu/bw-cli/internal/ui"

	"github.com/rivo/tview"
)

func main() {
	// Define a flag to capture the environment (e.g., bwstage)
	env := flag.String("env", "bwstaging", "AWS Vault environment (e.g. bwstaging, bwprod)")
	flag.Parse()

	// Fetch all service details (running and desired container counts)
	services, err := aws.GetAllServiceDetails(*env)
	if err != nil {
		log.Fatalf("Error fetching services: %v", err)
	}

	// Create a new tview application instance
	app := tview.NewApplication()

	// Display the fetched services in the terminal UI, passing the app and env variable
	ui.DisplayServices(app, services, *env)

	// Run the application
	if err := app.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}
