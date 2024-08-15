// Check if each container in the local machine's container list has an image that matches the latest version from the remote repository.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

type CacheMap map[string]ImageInfo

type GHCRVersion struct {
	Digest string `json:"name"` // startwith "sha256:"
	ApiUrl string `json:"url"`
}

var ghcr_token *string
var cache CacheMap

// Use registry APIs to fetch image info
func GetRemoteDockerInfo(image, tag, digest string) (ImageInfo, error) {
	// [registry-hostname]/[namespace]/[image-name]:[tag]
	var url string
	var info ImageInfo
	if v, ok := cache[image+":"+tag]; ok {
		return v, nil
	}

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

	headers := make(http.Header)

	switch registry {
	// https://github.com/rancher/image-mirror/blob/2528359b6681c2bbaaa1a2cd1c2db9005e8cbff1/retrieve-image-tags/retrieve-image-tags.py#L36
	case "docker.io":
		url = fmt.Sprintf("https://registry.hub.docker.com/v2/repositories/%s/%s/tags/%s", namespace, name, tag)
	case "ghcr.io":
		// https://docs.github.com/zh/rest/packages/packages?apiVersion=2022-11-28#list-package-versions-for-a-package-owned-by-an-organization
		if ghcr_token == nil {
			return info, fmt.Errorf("missing ghcr_token")
		}
		url = fmt.Sprintf("https://api.github.com/orgs/%s/packages/container/%s/versions", namespace, name)
		headers.Set("Accept", "application/vnd.github+json")
		headers.Set("Authorization", "Bearer "+*ghcr_token)
		headers.Set("X-GitHub-Api-Version", "2022-11-28")
	case "gcr.io":
		// url = "https://gcr.io/v2/{namespace}/{package}/tags/list"
		fallthrough
	case "quay.io":
		// url = "https://quay.io/api/v1/repository/{namespace}/{package}/tag/"
		fallthrough
	default:
		return info, fmt.Errorf("not support image %s", image)
	}

	for page := 1; ; page++ {
		params := ""
		if registry == "ghcr.io" {
			params = fmt.Sprintf("?page=%d&per_page=100", page)
		}

		log.Println("url:", url+params)

		req, err := http.NewRequest("GET", url+params, nil)
		if err != nil {
			return info, fmt.Errorf("error while creating request: %s", err)
		}

		req.Header = headers

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return info, fmt.Errorf("error while getting %s: %s", url, err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return info, fmt.Errorf("error while reading body: %s", err)
		}

		if registry == "docker.io" {
			err = json.Unmarshal(body, &info)
			if err != nil {
				return info, fmt.Errorf("server error while unmarshalling body: %s", err)
			}

			if info.MultiplePlatformImageInfoList == nil {
				return info, fmt.Errorf("error %s", string(body))
			} else if len(info.MultiplePlatformImageInfoList) == 0 {
				return info, fmt.Errorf("images is empty for %s:%s", image, tag)
			}

			cache[image+":"+tag] = info

			return info, nil
		} else if registry == "ghcr.io" {
			var resVersions []GHCRVersion
			err = json.Unmarshal(body, &resVersions)
			if err != nil {
				return info, fmt.Errorf("server error while unmarshalling body: %s", err)
			}
			// log.Println(resVersions)
			if len(resVersions) == 0 {
				return info, fmt.Errorf("no matching images for %s:%s", image, tag)
			}

			for _, v := range resVersions {
				if v.Digest == digest {
					log.Println("v.Digest", v.Digest, digest, "✅")
					// Get v.ApiUrl
					req, err := http.NewRequest("GET", v.ApiUrl, nil)
					if err != nil {
						return info, fmt.Errorf("error while creating request: %s", err)
					}

					req.Header = headers
					resp, err := client.Do(req)
					if err != nil {
						return info, fmt.Errorf("error while getting %s: %s", v.ApiUrl, err)
					}
					defer resp.Body.Close()
					body, err = io.ReadAll(resp.Body)
					if err != nil {
						return info, fmt.Errorf("error while reading body: %s", err)
					}
					// todo
					log.Println(string(body))
					return info, nil
				}
			}

		}
	}

}

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

func main() {
	// set up ghcr token from env
	v := os.Getenv("GHCR_TOKEN")
	if v != "" {
		ghcr_token = &v
	}
	// init cache map
	cache = make(CacheMap)

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
		latest, err := GetRemoteDockerInfo(imageName, "latest", imageDigest)
		if err != nil {
			log.Println("Unable to get remote docker tag:", name, imageName, err)
			continue
		}

		if imageDigest == latest.Digest {
			log.Println(name, imageName, imageTag, "✅")
			continue
		} else if imageTag == "latest" {
			log.Println(name, imageName, imageTag, "❌")
			continue
		}

		current, err := GetRemoteDockerInfo(imageName, imageTag, imageDigest)

		if err != nil {
			log.Println("Unable to get remote docker tag:", err)
		}

		var currentDigest string
		var latestDigest string

		for _, img := range current.MultiplePlatformImageInfoList {
			if img.OS == container.ImageInspect.Os && img.Architecture == container.ImageInspect.Architecture {
				currentDigest = img.Digest
			}
		}
		if currentDigest == "" {
			log.Println("Unable to find current digest for", container.ImageInspect.Os, container.ImageInspect.Architecture)
			continue
		}

		for _, img := range latest.MultiplePlatformImageInfoList {
			if img.OS == container.ImageInspect.Os && img.Architecture == container.ImageInspect.Architecture {
				latestDigest = img.Digest
			}
		}
		if latestDigest == "" {
			log.Println("Unable to find latest digest for", container.ImageInspect.Os, container.ImageInspect.Architecture)
			continue
		}

		if currentDigest != latestDigest {
			log.Println(name, imageName, imageTag, "❌")
		} else {
			log.Println(name, imageName, imageTag, "✅")
		}

	}

}
