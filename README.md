# bw-cli

`bw-cli` is a lightweight command-line interface for managing ECS services, inspired by K9s but designed specifically for AWS ECS environments. It allows you to interact with your ECS services, check service details, update service counts, restart services, and even shell into running containers using ECS Exec.

## Usage

Once installed, you can run `bw-cli` to interact with your ECS services directly from your terminal. Below are some key features and commands:

- **Shell into a container**: Press `s` to open a shell into a running container using ECS Exec.
- **Restart all containers**: Press `R` to redeploy all ECS containers in a selected service.
- **Update desired container count**: Select a service and change the desired number of tasks.
- **View services**: Get an overview of running services with details on desired and running task counts.

## Installation

You can install `bw-cli` using [Homebrew](https://brew.sh/). Follow these steps:

```
brew tap alexalbu001/homebrew-bw-cli
brew install bw-cli
```


### AWS Permissions

To use `bw-cli`, you must have the appropriate AWS permissions configured, including:
- ECS permissions to list clusters, services, and tasks.
- STS permissions to retrieve account information (`sts:GetCallerIdentity`).
- Permissions to execute commands in containers using ECS Exec (`ecs:ExecuteCommand`).

Ensure your AWS credentials are properly configured in your environment and the permissions are set in the IAM role or user you're using.


### ECS Task Definitions

In order to use the **ECS Exec** feature (`s` to shell into a container), the ECS task definitions must have the `enableExecuteCommand` flag enabled. To do this, you need to ensure the following in your task definition:

```json
"containerDefinitions": [
  {
    "name": "your-container",
    "image": "your-image",
    ...
    "command": [...],
    "essential": true,
    "enableExecuteCommand": true
  }
]
```

## Development

For local development or contributing:

1. Clone the repository:
   `git clone https://github.com/alexalbu001/bw-cli.git`
2. Install dependencies:
   `go mod tidy`
3. Build the project:
   `go build -o bw-cli`

## License

This project is licensed under the MIT License.
