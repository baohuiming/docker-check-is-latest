package main

import "github.com/docker/docker/api/types"

type MultiplePlatformImageInfo struct {
	Digest       string `json:"digest"`
	Os           string `json:"os"`
	Architecture string `json:"architecture"`
}

type ImageInfo struct {
	Digest                        string
	MultiplePlatformImageInfoList []MultiplePlatformImageInfo // for docker.io
	Tags                          []string
}

type Container struct {
	types.Container
	ImageInspect types.ImageInspect
}

type Cache struct {
	ImageInfoCache map[string]ImageInfo
	HTTPCache      map[string][]byte
}

type GHCRVersionResp struct {
	Digest   string `json:"name"` // startwith "sha256:"
	Metadata struct {
		Container struct {
			Tags []string `json:"tags"`
		} `json:"container"`
	} `json:"metadata"`
}

type DockerHubTagResp struct {
	Digest                        string                      `json:"digest"` // sha256:7a3e18f29578feb271bb8daab4379e4ebd355b87ea64b699ce74e6ff49d907aa
	Tag                           string                      `json:"name"`   // latest
	MultiplePlatformImageInfoList []MultiplePlatformImageInfo `json:"images"`
}

type DockerHubTagsResp struct {
	Results []DockerHubTagResp `json:"results"`
}

type CheckResult struct {
	Container  string `json:"container"`
	Image      string `json:"image"`
	IsLatest   string `json:"is_latest"`
	LatestTags string `json:"latest_tags"`
}
