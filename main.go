// Check if each container in the local machine's container list has an image that matches the latest version from the remote repository.
// Portainer's similar script ref: https://github.com/portainer/portainer/blob/054898f821544e16c58ec0595c8990e0d2d415bf/api/docker/images/status.go
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
)

var (
	ghcr_token   string
	outputPath   string
	cache        Cache
	checkResults []CheckResult
	proxy        string
	transport    *http.Transport = &http.Transport{}
)

func check(containerName, imageName, isLatest, latestTags string) {
	log.Printf("%10s %s %s {%s}", "["+isLatest+"]", containerName, imageName, latestTags)
	if outputPath != "" {
		checkResults = append(checkResults, CheckResult{containerName, imageName, isLatest, latestTags})
	}
}

func main() {
	// set up ghcr token from flag
	flag.StringVar(&ghcr_token, "ghcr_token", "", "GitHub Container Registry token")
	flag.StringVar(&outputPath, "output", "", "Output file path")
	flag.StringVar(&proxy, "proxy", "", "Proxy URL")
	flag.Parse()

	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			log.Fatal("Unable to parse proxy URL:", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	// init cache
	cache = Cache{
		ImageInfoCache: make(map[string]ImageInfo),
		HTTPCache:      make(map[string][]byte),
	}

	containers, err := GetLocalContainers()
	if err != nil {
		log.Fatal("Unable to get docker list:", err)
	}

	for _, container := range containers {
		// set default value
		name := container.Names[0]   // e.g. /qdrant
		imageName := container.Image // e.g. qdrant/qdrant
		registry := "docker.io"
		if imagePart := strings.Split(imageName, "/"); len(imagePart) > 2 {
			registry = imagePart[len(imagePart)-3]
		}
		imageTag := "latest"
		if strings.Contains(imageName, ":") {
			imageTag = strings.Split(imageName, ":")[1]
			imageName = strings.Split(imageName, ":")[0]
		}

		var latestInfo ImageInfo
		var currentInfo ImageInfo

		latestInfo, err = GetRemoteImageInfo(imageName, "latest", nil)
		if err != nil { // unable to get latest info, 404 or other error
			log.Println("Unable to get remote docker tag:", name, imageName, err)
			check(name, imageName+":"+imageTag, "unknown", "")
			continue
		}

		if slices.Contains(container.ImageInspect.RepoDigests, imageName+"@"+latestInfo.Digest) {
			check(name, imageName+":"+imageTag, "yes", strings.Join(latestInfo.Tags, "|"))
			continue
		} else if registry == "docker.io" && imageTag == "latest" {
			check(name, imageName+":"+imageTag, "no", "")
			continue
		}

		currentInfo, err := GetRemoteImageInfo(imageName, imageTag, container.ImageInspect.RepoDigests)

		if err != nil {
			log.Println("Unable to get remote docker tag:", err)
			check(name, imageName+":"+imageTag, "unknown", "")
			continue
		}

		if registry == "ghcr.io" {
			if slices.Contains(currentInfo.Tags, "latest") {
				check(name, imageName+":"+imageTag, "yes", strings.Join(latestInfo.Tags, "|"))
			} else {
				check(name, imageName+":"+imageTag, "no", strings.Join(latestInfo.Tags, "|"))
			}
			continue
		}

		if registry == "docker.io" {
			var currentDigest string
			var latestDigest string

			for _, img := range currentInfo.MultiplePlatformImageInfoList {
				if img.OS == container.ImageInspect.Os && img.Architecture == container.ImageInspect.Architecture {
					currentDigest = img.Digest
				}
			}
			if currentDigest == "" {
				log.Println("Unable to find current digest for", container.ImageInspect.Os, container.ImageInspect.Architecture)
				check(name, imageName+":"+imageTag, "unknown", "")
				continue
			}

			for _, img := range latestInfo.MultiplePlatformImageInfoList {
				if img.OS == container.ImageInspect.Os && img.Architecture == container.ImageInspect.Architecture {
					latestDigest = img.Digest
				}
			}
			if latestDigest == "" {
				log.Println("Unable to find latest digest for", container.ImageInspect.Os, container.ImageInspect.Architecture)
				check(name, imageName+":"+imageTag, "unknown", "")
				continue
			}

			if currentDigest != latestDigest {
				check(name, imageName+":"+imageTag, "no", "")
				continue
			} else {
				check(name, imageName+":"+imageTag, "yes", "")
				continue
			}
		}

		check(name, imageName+":"+imageTag, "unknown", "")
	}

	// write output to file
	if outputPath != "" {
		jsonData, err := json.MarshalIndent(checkResults, "", "  ")
		if err != nil {
			log.Fatal("Unable to marshal json:", err)
			return
		}

		err = os.WriteFile(outputPath, jsonData, os.ModePerm)
		if err != nil {
			log.Fatal("Unable to write file:", err)
		}
	}
}
