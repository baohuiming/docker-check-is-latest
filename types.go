package main

import "github.com/docker/docker/api/types"

type MultiplePlatformImageInfo struct {
	Digest       string `json:"digest"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

type ImageInfo struct {
	Digest                        string                      `json:"digest"`
	MultiplePlatformImageInfoList []MultiplePlatformImageInfo `json:"images"` // for docker.io
	Tags                          []string                    // for ghcr.io
}

type Container struct {
	types.Container
	ImageInspect types.ImageInspect
}

type Cache struct {
	ImageInfoCache map[string]ImageInfo
	HTTPCache      map[string][]byte
}

type GHCRVersion struct {
	Digest   string `json:"name"` // startwith "sha256:"
	Metadata struct {
		Container struct {
			Tags []string `json:"tags"`
		} `json:"container"`
	} `json:"metadata"`
}

type CheckResult struct {
	Container  string `json:"container"`
	Image      string `json:"image"`
	IsLatest   string `json:"is_latest"`
	LatestTags string `json:"latest_tags"`
}
