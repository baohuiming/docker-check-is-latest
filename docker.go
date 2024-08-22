package main

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// Use docker client API to fetch portainer list
func GetDockerPortainerList() ([]Container, error) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error while creating docker client: %s", err)
	}

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})

	if err != nil {
		return nil, fmt.Errorf("error while listing containers: %s", err)
	}

	containerWithImageInfos := make([]Container, 0, len(containers))
	for _, c := range containers {
		img, _, err := cli.ImageInspectWithRaw(ctx, c.Image)
		if err != nil {
			return nil, fmt.Errorf("error while inspecting image %s of container %s: %s", c.Image, c.ID, err)
		}

		containerWithImageInfo := Container{
			Container:    c,
			ImageInspect: img,
		}

		containerWithImageInfos = append(containerWithImageInfos, containerWithImageInfo)
	}
	return containerWithImageInfos, nil
}
