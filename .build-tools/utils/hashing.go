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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// HashDirectory returns a hash that is based on the hash of each file in that directory.
// If a prefix is given, it's included in the string to hash, at the beginning.
func HashDirectory(basePath string, ignores *gitignore.GitIgnore, prefix string) (string, error) {
	// Compute the hash of the app's files
	files := []string{}
	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		// Skip folders and ignored files
		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}
		if relPath == "." ||
			relPath == ContainerConfigFile ||
			d.IsDir() ||
			(ignores != nil && ignores.MatchesPath(path)) {
			return nil
		}

		// Compute the sha256 of the file
		checksum, err := ChecksumFile(path)
		if err != nil {
			return err
		}

		// Convert all slashes to / so the hash is the same on Windows and Linux
		relPath = filepath.ToSlash(relPath)

		files = append(files, relPath+" "+checksum)
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no file found in the folder")
	}

	// Sort files to have a consistent order, then compute the checksum of that string (getting the first 10 chars only)
	sort.Strings(files)
	fileList := strings.Join(files, "\n")
	hashDir := ChecksumString(prefix + fileList)[0:10]

	return hashDir, nil
}

// GetIgnores loads the ".gitignore" file(s) to exclude files from calculating the checksum of a folder.
func GetIgnores(files []string) *gitignore.GitIgnore {
	lines := []string{}
	for _, f := range files {
		read, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines = append(lines, strings.Split(string(read), "\n")...)
	}

	if len(lines) == 0 {
		return nil
	}

	return gitignore.CompileIgnoreLines(lines...)
}

// ChecksumFile calculates the SHA256 checksum of a file.
func ChecksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("failed to open file %s for hashing: %v\n", path, err)
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		fmt.Printf("failed to copy file %s into hasher: %v\n", path, err)
		return "", err
	}

	res := hex.EncodeToString(h.Sum(nil))
	return res, nil
}

// ChecksumString calculates the SHA256 checksum of a string.
func ChecksumString(str string) string {
	h := sha256.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}
