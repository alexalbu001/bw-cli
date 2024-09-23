package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alexalbu001/bw-cli/internal/aws"
	"github.com/alexalbu001/bw-cli/internal/ui"

	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

var (
	version string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "bw-cli",
	Short: "bw-cli is a command-line interface for managing AWS ECS services",
	Long: `bw-cli is a command-line tool that provides an interactive terminal UI 
for managing and monitoring AWS ECS services. It allows users to view service 
details, update desired counts, and perform other ECS-related operations.`,
	Run: func(cmd *cobra.Command, args []string) {
		runCLI()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of bw-cli",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("bw-cli version %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runCLI() {
	err := aws.GetCallerIdentity()
	if err != nil {
		log.Fatalf("Error fetching AWS identity: %v", err)
	}

	services, err := aws.GetAllServiceDetails()
	if err != nil {
		log.Fatalf("Error fetching services: %v", err)
	}

	app := tview.NewApplication()
	ui.DisplayServices(app, services)

	if err := app.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}
