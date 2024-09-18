# bw-cli - ECS Service Scaling Tool

## bw-cli is a simple CLI tool that allows users to view and modify the desired count of ECS services running in different clusters. The tool leverages the AWS CLI and aws-vault to manage credentials securely, and it provides an interactive, terminal-based user interface built with tview.

## Prerequisites
To run bw-cli, you will need the following installed on your machine:

Go: Install Go (version 1.18 or above).
AWS CLI: Install AWS CLI.
aws-vault: Install aws-vault for managing AWS credentials.
tview: The app uses tview for the terminal-based UI, which is installed automatically with Go modules.

### Installation - Using Homebrew


brew tap alexalbu001/bw-cli
brew install bw-cli

bw-cli --help

### Testing - WIP