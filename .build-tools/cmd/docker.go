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

package cmd

import (
	"build-tools/utils"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	// dockerCmd represents the docker command
	var dockerCmd = &cobra.Command{
		Use:   "docker",
		Short: "Tools for Docker images",
		Long:  `Build and push Docker images`,
	}
	rootCmd.AddCommand(dockerCmd)

	dockerCmd.AddCommand(baseImagesHashCmd())
}

// "hash" sub-command
func baseImagesHashCmd() *cobra.Command {
	var (
		flagName     string
		flagDir      string
		flagPlatform string
	)

	cmd := &cobra.Command{
		Use:   "hash",
		Short: "Calculates the hash of an image",
		Long:  "Calculates the hash of the content of a Docker image (before it's built)",
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(flagDir, flagName)
			dockerfile := filepath.Join(baseDir, "Dockerfile")

			// Ensure the image exists
			// Only images that have a sub-folder inside "docker" are supported by this tool
			exists, err := utils.Exists(dockerfile)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("could not find a Dockerfile at %s", dockerfile)
			}

			// Check if we have a container config file
			config, err := utils.FindContainerConfigInFolder(baseDir)
			if err != nil {
				return err
			}

			// Compute the hash of the image
			hash, err := utils.HashDockerImage(baseDir, flagPlatform, config)
			if err != nil {
				return err
			}
			fmt.Println(hash)

			return nil
		},
	}

	cmd.Flags().StringVarP(&flagName, "name", "n", "", "Name of the image")
	cmd.MarkFlagRequired("name")
	cmd.Flags().StringVarP(&flagDir, "dir", "d", "docker", "Base directory for Dockerfiles")
	cmd.Flags().StringVarP(&flagPlatform, "platform", "p", "linux/amd64", "Platform for the Docker image")

	return cmd
}
