package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// Use docker client API to fetch local containers
func GetLocalContainers() ([]Container, error) {
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

// Parse image name to get registry, namespace and name
func parseImageName(image string) (registry string, namespace string, name string) {
	// image: [registry-hostname]/[namespace]/[image-name]
	// check number of "/" in image
	imagePart := strings.Split(image, "/")
	imagePartLen := len(imagePart)
	registry = "docker.io"
	namespace = "library"
	name = imagePart[imagePartLen-1]

	if imagePartLen >= 2 { // e.g. esphome/esphome
		namespace = imagePart[imagePartLen-2]
	}
	if imagePartLen >= 3 { // e.g. m.daocloud.io/ghcr.io/esphome/esphome
		registry = imagePart[imagePartLen-3]
	}
	return registry, namespace, name
}

func sendRequest(url string, headers map[string]string) (body []byte, err error) {
	if b, ok := cache.HTTPCache[url]; ok {
		log.Println("cache hit", url)
		return b, nil
	} else {
		header := make(http.Header)
		for k, v := range headers {
			header.Set(k, v)
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return body, fmt.Errorf("error while creating request: %s", err)
		}

		req.Header = header

		client := &http.Client{
			Transport: transport,
		}
		resp, err := client.Do(req)
		if err != nil {
			return body, fmt.Errorf("error while getting %s: %s", url, err)
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return body, fmt.Errorf("error while reading body: %s", err)
		}

		cache.HTTPCache[url] = body
		return body, nil
	}
}

func getDockerHubImageAllTags(image string, digest string) (tags []string, err error) {
	// digest: sha256:7a3e...
	_, namespace, name := parseImageName(image)
	url := fmt.Sprintf("https://registry.hub.docker.com/v2/repositories/%s/%s/tags", namespace, name)
	headers := make(map[string]string)
	for page := 1; ; page++ {
		params := fmt.Sprintf("?page=%d&page_size=100", page)

		log.Println("GET", url+params)

		body, err := sendRequest(url+params, headers)
		if err != nil {
			return nil, err
		}

		var respTags DockerHubTagsResp
		err = json.Unmarshal(body, &respTags)
		if err != nil {
			return nil, fmt.Errorf("server error while unmarshalling body: %s", err)
		}

		if len(respTags.Results) == 0 {
			return tags, nil
		}

		pageContains := false
		for _, t := range respTags.Results {
			if t.Digest == digest {
				tags = append(tags, t.Tag)
				pageContains = true
			}
		}

		if !pageContains {
			return tags, nil
		}
	}
}

// Use registry APIs to fetch image info
// if allTag is true, fetch all tags of the image, just for docker.io
func GetRemoteImageInfo(image string, tag string, digests []string, allTag bool) (ImageInfo, error) {
	var url string
	var info ImageInfo
	allTag = true
	if v, ok := cache.ImageInfoCache[image+":"+tag+strings.Join(digests, ",")]; ok {
		return v, nil
	}
	registry, namespace, name := parseImageName(image)
	headers := make(map[string]string)
	switch registry {
	// ref: https://github.com/rancher/image-mirror/blob/2528359b6681c2bbaaa1a2cd1c2db9005e8cbff1/retrieve-image-tags/retrieve-image-tags.py#L36
	case "docker.io":
		// doc: https://docs.docker.com/reference/api/hub/latest/#tag/repositories
		url = fmt.Sprintf("https://registry.hub.docker.com/v2/repositories/%s/%s/tags/%s", namespace, name, tag)
	case "ghcr.io":
		// doc: https://docs.github.com/zh/rest/packages/packages?apiVersion=2022-11-28#list-package-versions-for-a-package-owned-by-an-organization
		if ghcr_token == "" {
			return info, fmt.Errorf("missing ghcr_token")
		}
		url = fmt.Sprintf("https://api.github.com/orgs/%s/packages/container/%s/versions", namespace, name)
		headers["Accept"] = "application/vnd.github+json"
		headers["Authorization"] = "Bearer " + ghcr_token
		headers["X-GitHub-Api-Version"] = "2022-11-28"
	case "gcr.io":
		// url = "https://gcr.io/v2/{namespace}/{package}/tags/list"
		fallthrough
	case "quay.io":
		// url = "https://quay.io/api/v1/repository/{namespace}/{package}/tag/"
		fallthrough
	default:
		return ImageInfo{}, fmt.Errorf("not support image %s", image)
	}

	for page := 1; ; page++ {
		params := ""
		if registry == "ghcr.io" {
			params = fmt.Sprintf("?page=%d&per_page=100", page)
		}
		log.Println("GET", url+params)

		body, err := sendRequest(url+params, headers)
		if err != nil {
			return ImageInfo{}, err
		}

		if registry == "docker.io" {
			var respTag DockerHubTagResp
			err := json.Unmarshal(body, &respTag)
			if err != nil {
				return ImageInfo{}, fmt.Errorf("server error while unmarshalling body: %s", err)
			}
			info.Digest = respTag.Digest
			info.MultiplePlatformImageInfoList = respTag.MultiplePlatformImageInfoList

			if info.MultiplePlatformImageInfoList == nil {
				return ImageInfo{}, fmt.Errorf("error %s", string(body))
			} else if len(info.MultiplePlatformImageInfoList) == 0 {
				return ImageInfo{}, fmt.Errorf("error images is empty for %s:%s", image, tag)
			}

			if allTag {
				tags, err := getDockerHubImageAllTags(image, info.Digest)
				if err != nil {
					return ImageInfo{}, err
				}
				info.Tags = tags
			}

			cache.ImageInfoCache[image+":"+tag] = info

			return info, nil
		} else if registry == "ghcr.io" {
			var resVersions []GHCRVersionResp
			err := json.Unmarshal(body, &resVersions)
			if err != nil {
				return ImageInfo{}, fmt.Errorf("server error while unmarshalling body: %s", err)
			}

			if len(resVersions) == 0 {
				return ImageInfo{}, fmt.Errorf("no matching images for %s:%s %s %s", image, tag, url+params, string(body))
			}

			for _, v := range resVersions {
				if (digests != nil && slices.Contains(digests, image+"@"+v.Digest)) ||
					(digests == nil && slices.Contains(v.Metadata.Container.Tags, tag)) {
					info.Digest = v.Digest
					info.Tags = v.Metadata.Container.Tags
					cache.ImageInfoCache[image+":"+tag] = info

					return info, nil
				}
			}
		}
	}
}
