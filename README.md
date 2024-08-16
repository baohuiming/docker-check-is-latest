# docker-check-is-latest

A Go script that checks if the local container images are up-to-date with their corresponding `latest` tags on Docker Hub (docker.io) and GitHub Container Registry (ghcr.io).

## Usage

To use this script, simply run it in your terminal or command prompt. The script will automatically scan all running containers and compare their images with the `latest` versions available on Docker Hub and GitHub Container Registry.

```bash
go run main.go
```

### Command Line Arguments

You can specify two optional command line arguments:

1. **ghcr_token**: If you want to access private repositories on GitHub Container Registry, you need to provide a personal access token (PAT) with the necessary permissions. You can do this by setting the `ghcr_token` argument.

   ```bash
   go run main.go --ghcr_token=your_token_here
   ```

2. **output**: By default, the script will print the results to the console. However, if you want to save the results to a JSON file, you can set the `output` argument to the desired file path.

   ```bash
   go run main.go --output=/path/to/output.json
   ```

## Output

The script will output a list of containers and their image tags. For each container, it will indicate whether the local image is up-to-date with the `latest` version on Docker Hub and GitHub Container Registry.

If the `--output` argument is provided, the results will be saved to a JSON file in the following format:

```json
[
  {
    "container": "/thirsty_darwin",
    "image": "ghcr.io/esphome/esphome:2024.6",
    "is_latest": "no"
  },
  {
    "container": "/peaceful_jemison",
    "image": "eclipse-mosquitto:2.0.17",
    "is_latest": "no"
  },
  {
    "container": "/practical_bardeen",
    "image": "eclipse-mosquitto:2.0.18",
    "is_latest": "yes"
  },
  {
    "container": "/friendly_gagarin",
    "image": "ghcr.io/esphome/esphome:2024.7.3",
    "is_latest": "yes"
  },
  {
    "container": "/cool_robinson",
    "image": "m.daocloud.io/docker.io/nodered/node-red:latest",
    "is_latest": "yes"
  },
  {
    "container": "/friendly_engelbart",
    "image": "louislam/dockge:1",
    "is_latest": "yes"
  },
  {
    "container": "/busy_thompson",
    "image": "postgres:16.2",
    "is_latest": "no"
  },
  {
    "container": "/sleepy_heisenberg",
    "image": "postgres:16.2",
    "is_latest": "no"
  },
  {
    "container": "/agitated_engelbart",
    "image": "postgres:16.2",
    "is_latest": "no"
  },
  {
    "container": "/magical_hawking",
    "image": "postgres:16.2",
    "is_latest": "no"
  }
]
```

## Requirements

This script requires the following:

- Go version 1.22 (the version used for development and testing)
- Docker installed and running on your machine
- MacOS/Linux (the platform on which the script has been tested)

## License

This project is licensed under the MIT License.