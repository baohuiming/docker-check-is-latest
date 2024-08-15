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

type MultiplePlatformImageInfo struct {
	Digest       string `json:"digest"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

type ImageInfo struct {
	Digest                        string                      `json:"digest"`
	MultiplePlatformImageInfoList []MultiplePlatformImageInfo `json:"images"`
}

type Container struct {
	types.Container
	ImageInspect types.ImageInspect
}

// Use registry APIs to fetch image info
func GetRemoteDockerInfo(image, tag string) (ImageInfo, error) {
	// [registry-hostname]/[namespace]/[image-name]:[tag]
	var url string
	// check number of "/" in image
	imagePart := strings.Split(image, "/")
	imagePartLen := len(imagePart)
	var registry string = "docker.io"
	var namespace string = "library"
	var name string = imagePart[imagePartLen-1]

	if imagePartLen >= 2 {
		namespace = imagePart[imagePartLen-2]
	}
	if imagePartLen >= 3 { // e.g. m.daocloud.io/ghcr.io/esphome/esphome
		registry = imagePart[imagePartLen-3]
	}

	if registry == "ghcr.io" {
		url = fmt.Sprintf("https://ghcr.io/v2/%s/%s/manifests/%s", namespace, name, tag)
	} else if registry == "docker.io" {
		url = fmt.Sprintf("https://registry.hub.docker.com/v2/repositories/%s/%s/tags/%s", namespace, name, tag)
	} else {
		return ImageInfo{}, fmt.Errorf("not support image %s", image)
	}
	log.Println(url)

	// via docker hub
	resp, err := http.Get(url)
	if err != nil {
		return ImageInfo{}, fmt.Errorf("error while getting %s: %s", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ImageInfo{}, fmt.Errorf("error while reading body: %s", err)
	}

	var info ImageInfo
	err = json.Unmarshal(body, &info)
	if err != nil {
		return ImageInfo{}, fmt.Errorf("server error while unmarshalling body: %s", err)
	}

	if info.MultiplePlatformImageInfoList == nil {
		return ImageInfo{}, fmt.Errorf("error %s", string(body))
	} else if len(info.MultiplePlatformImageInfoList) == 0 {
		return ImageInfo{}, fmt.Errorf("images is empty for %s:%s", image, tag)
	}

	return info, nil
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

func main() {
	containers, err := GetDockerPortainerList()
	if err != nil {
		log.Fatal("Unable to get docker list:", err)
	}
	for _, container := range containers {
		name := container.Names[0]
		imageName := container.Image
		imageDigest := strings.Split(container.ImageInspect.RepoDigests[0], "@")[1] // startwith "sha256:"
		var imageTag string = "latest"
		if strings.Contains(imageName, ":") {
			imageTag = strings.Split(imageName, ":")[1]
			imageName = strings.Split(imageName, ":")[0]
		}
		log.Println("Handling container:", name, imageName)
		latest, err := GetRemoteDockerInfo(imageName, "latest")
		if err != nil {
			log.Println("Unable to get remote docker tag:", err)
			continue
		}

		if imageDigest == latest.Digest {
			log.Println("Image is up to date:", name, imageName)
			continue
		} else if imageTag == "latest" {
			log.Println("Image is not up to date:", name, imageName)
			continue
		}

		current, err := GetRemoteDockerInfo(imageName, imageTag)
		// 比较两个MultiplePlatformImageInfoList是否

		if err != nil {
			log.Println("Unable to get remote docker tag:", err)
		}
		log.Println(current, latest)
		// todo
	}

}
