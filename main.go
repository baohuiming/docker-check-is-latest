package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Images []struct {
	Digest string `json:"digest"`
}

type ImageDetail struct {
	Digest string `json:"digest"`
	Images Images `json:"images"`
}

type Container struct {
	types.Container
	ImageDigest string
}

func GetRemoteDockerTag(image, tag string) (Images, error) {
	var url string
	// check number of "/" in image
	imagePart := strings.Split(image, "/")
	imagePartLen := len(imagePart)
	switch len(imagePart) {
	case 1:
		image = "library/" + image
		url = "https://registry.hub.docker.com/v2/repositories/" + image + "/tags/" + tag
	case 2:
		url = "https://registry.hub.docker.com/v2/repositories/" + image + "/tags/" + tag
	default: // e.g. m.daocloud.io/ghcr.io/esphome/esphome
		if imagePart[imagePartLen-3] == "ghcr.io" {
			url = "https://ghcr.io/v2/" + image + "/manifests/" + tag
		} else {
			return nil, fmt.Errorf("not support image %s", image)
		}
	}

	// via docker hub
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error while getting %s: %s", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading body: %s", err)
	}

	var dockerTag ImageDetail
	err = json.Unmarshal(body, &dockerTag)
	if err != nil {
		return nil, fmt.Errorf("server error while unmarshalling body: %s", err)
	}

	if dockerTag.Images == nil {
		return nil, fmt.Errorf("error %s", string(body))
	} else if len(dockerTag.Images) == 0 {
		return nil, fmt.Errorf("images is empty for %s:%s", image, tag)
	}

	return dockerTag.Images, nil
}

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

	containerWithDigests := make([]Container, 0, len(containers))
	for _, c := range containers {
		img, _, err := cli.ImageInspectWithRaw(ctx, c.Image)
		if err != nil {
			return nil, fmt.Errorf("error while inspecting image %s of container %s: %s", c.Image, c.ID, err)
		}

		containerWithDigest := Container{
			Container:   c,
			ImageDigest: strings.Split(img.RepoDigests[0], "@")[1],
		}

		containerWithDigests = append(containerWithDigests, containerWithDigest)
	}
	return containerWithDigests, nil

}

func main() {
	containers, err := GetDockerPortainerList()
	if err != nil {
		log.Fatal("Unable to get docker list:", err)
	}
	for _, container := range containers {
		name := container.Names[0]
		imageName := container.Image
		imageDigest := container.ImageDigest
		if strings.Contains(imageName, ":") {
			imageName = strings.Split(imageName, ":")[0]
		}
		log.Println("Handling container:", name, imageName)
		images, err := GetRemoteDockerTag(imageName, "latest")
		if err != nil {
			log.Println("Unable to get remote docker tag:", err)
			continue
		}

		// search imageId in images
		found := false
		for _, image := range images {
			if image.Digest == imageDigest {
				log.Println("Image is up to date:", name, imageName)
				found = true
			} else {
				log.Println(image.Digest, "!=", imageDigest)
			}
		}
		if !found {
			log.Println("Image is outdated:", name, imageName)
		}
	}

}
