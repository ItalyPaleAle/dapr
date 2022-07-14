/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	crName "github.com/google/go-containerregistry/pkg/name"
	crV1 "github.com/google/go-containerregistry/pkg/v1"
	crRemote "github.com/google/go-containerregistry/pkg/v1/remote"
	ignore "github.com/sabhiram/go-gitignore"
)

// Name of the config file for a container.
const ContainerConfigFile = ".container.json"

// Contents of the container config file.
type ContainerConfig struct {
	RemoteImagesHash []string `json:"RemoteImagesHash"`
	IgnoreFromFile   string   `json:"IgnoreFromFile"`
}

// FindContainerConfigInFolder loads the container config file in the folder if it exists. Returns nil and no error if there's no container config file.
func FindContainerConfigInFolder(folder string) (*ContainerConfig, error) {
	res, err := LoadContainerConfig(filepath.Join(folder, ContainerConfigFile))
	if err != nil && os.IsNotExist(err) {
		res = nil
		err = nil
	}
	return res, err
}

// LoadContainerConfig loads the container config file.
func LoadContainerConfig(path string) (*ContainerConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	res := &ContainerConfig{}
	err = json.NewDecoder(f).Decode(res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// HashDockerImage returns the hash of the files that will be part of a Docker image (before building it).
func HashDockerImage(basePath string, platform string, config *ContainerConfig) (string, error) {
	if config == nil {
		config = &ContainerConfig{}
	}

	// Load the ".gitignore" if we have it
	var ignores *ignore.GitIgnore
	if config.IgnoreFromFile != "" {
		ignores = GetIgnores([]string{
			filepath.Join(basePath, config.IgnoreFromFile),
		})
	}

	// If we also have remote base images, get their hash
	var prefix string
	if len(config.RemoteImagesHash) > 0 {
		platformObj, err := crV1.ParsePlatform(platform)
		if err != nil {
			return "", err
		}
		for _, ri := range config.RemoteImagesHash {
			ref, err := crName.ParseReference(ri)
			if err != nil {
				return "", err
			}
			image, err := crRemote.Image(ref, crRemote.WithPlatform(*platformObj))
			if err != nil {
				return "", err
			}
			digest, err := image.Digest()
			if err != nil {
				return "", err
			}
			fmt.Fprintf(os.Stderr, "Found base image '%s' with hash '%s'\n", ri, digest.String())
			prefix += fmt.Sprintf("BASEIMG: %s: %s\n", ri, digest.String())
		}

		prefix += "\n"
	}

	// Compute the hash of the files in the directory
	dirHash, err := HashDirectory(basePath, ignores, prefix)
	if err != nil {
		return "", err
	}

	return dirHash, nil
}
