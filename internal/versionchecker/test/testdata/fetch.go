/*
Copyright 2021 The cert-manager Authors.

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

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"

	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
)

const minVersion = "v1.0.0"

const repoURL = "https://github.com/cert-manager/cert-manager"
const downloadURL = "https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml"

const dummyVersion = "v99.99.99"

func main() {
	ctx := context.Background()
	stdOut := os.Stdout

	if len(os.Args) != 3 && len(os.Args) != 4 {
		fmt.Fprintf(stdOut, "Usage: %s <test_manifests.yaml> <max_version> [<force:bool>]\n", os.Args[0])
		os.Exit(1)
	}

	manifestsPath := os.Args[1]
	maxVersion := os.Args[2]
	force := false
	if len(os.Args) == 4 {
		if strings.ToLower(os.Args[3])[0] == 't' {
			force = true
		}
	}

	// Read the inventory file
	var inv Inventory
	if err := inv.read(manifestsPath); err != nil {
		fmt.Fprintf(stdOut, "Error reading inventory: %v\n", err)

		inv.reset()
	}

	// If the passed version is identical to the latest version, we don't need to do anything
	if inv.LatestVersion == maxVersion && !force {
		fmt.Fprintf(stdOut, "Version %s is already the latest version\n", maxVersion)
		os.Exit(0)
	}

	// Fetch the list of remote versions
	remoteVersions, err := listVersions(ctx, maxVersion)
	if err != nil {
		fmt.Fprintf(stdOut, "Error listing versions: %v\n", err)
		os.Exit(1)
	}

	// List the remote versions that are not in the inventory
	newVersions := make([]string, 0, len(remoteVersions))
	for version := range remoteVersions {
		if _, ok := inv.Versions[version]; !ok || force {
			newVersions = append(newVersions, version)
		}
	}

	// Download the new versions
	type versionManifest struct {
		version  string
		manifest []byte
	}
	results := make(chan versionManifest, len(newVersions))
	group, gctx := errgroup.WithContext(ctx)
	for _, version := range newVersions {
		version := version
		group.Go(func() error {
			manifests, err := downloadManifests(gctx, version)
			if err != nil {
				return fmt.Errorf("error downloading CRD for version %s: %v", version, err)
			}

			// Cleanup the manifests
			manifests, err = cleanupManifests(manifests, version)
			if err != nil {
				return fmt.Errorf("error cleaning up manifests for version %s: %v", version, err)
			}

			results <- versionManifest{
				version:  version,
				manifest: manifests,
			}

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		fmt.Fprintf(stdOut, "Error downloading manifests: %v\n", err)
		os.Exit(1)
	}

	close(results)

	for result := range results {
		hash, err := manifestHash(result.manifest)
		if err != nil {
			fmt.Fprintf(stdOut, "Error hashing manifest: %v\n", err)
			os.Exit(1)
		}

		inv.Versions[result.version] = hash
		inv.Manifests[hash] = result.manifest
	}

	// Update the latest version
	inv.LatestVersion = maxVersion

	// Write the inventory file
	if err := inv.write(manifestsPath); err != nil {
		fmt.Fprintf(stdOut, "Error writing inventory: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(stdOut, "Updated inventory to version %s\n", maxVersion)
}

type Inventory struct {
	LatestVersion string

	Versions map[string]string

	Manifests map[string][]byte
}

func (inv *Inventory) reset() {
	*inv = Inventory{
		LatestVersion: "v0.0.0",
		Versions:      make(map[string]string),
		Manifests:     make(map[string][]byte),
	}
}

func (inv *Inventory) read(manifestsPath string) error {
	inv.reset()

	// Read the inventory file
	manfestsBytes, err := os.ReadFile(manifestsPath)
	if err != nil {
		return fmt.Errorf("failed to read inventory file: %v", err)
	}

	// Read latest version from first line
	fileSplit := bytes.SplitN(manfestsBytes, []byte("\n"), 2)
	if len(fileSplit) != 2 {
		return fmt.Errorf("failed read latest version from first line in manifest file")
	}

	latestVersion := string(fileSplit[0])
	latestVersion = strings.TrimSpace(latestVersion)
	latestVersion = strings.TrimPrefix(latestVersion, "# [CHK_LATEST_VERSION]: ")
	latestVersion = semver.Canonical(latestVersion)

	if latestVersion == "" {
		return fmt.Errorf("failed to parse latest version from first line in manifest file")
	}

	inv.LatestVersion = latestVersion

	// Split the rest of the file into the manifests
	manfestsBytes = fileSplit[1]

	manifests := bytes.Split(manfestsBytes, []byte("---\n# [CHK_VERSIONS]: "))

	for _, manifest := range manifests {
		if len(manifest) == 0 {
			continue
		}

		parts := bytes.SplitN(manifest, []byte("\n"), 2)
		if len(parts) != 2 {
			return fmt.Errorf("failed to read versions from manifest file")
		}

		versions := string(parts[0])
		versions = strings.TrimSpace(versions)

		manifest = parts[1]
		manifest, err = cleanupManifests(manifest, dummyVersion)
		if err != nil {
			return fmt.Errorf("failed to cleanup manifest: %v", err)
		}

		manifestHasher := fnv.New64()
		if _, err := manifestHasher.Write(manifest); err != nil {
			return fmt.Errorf("failed to hash manifest: %v", err)
		}

		manifestHash := hex.EncodeToString(manifestHasher.Sum([]byte{}))

		// Split the versions
		versionsSplit := strings.Split(versions, ",")
		for _, version := range versionsSplit {
			version = strings.TrimSpace(version)
			version = semver.Canonical(version)

			if version == "" {
				return fmt.Errorf("failed to parse version from manifest file")
			}

			inv.Versions[version] = manifestHash
		}

		if len(inv.Versions) > 0 {
			inv.Manifests[manifestHash] = manifest
		}
	}

	return nil
}

func (inv *Inventory) write(manifestsPath string) error {
	// Write the inventory file
	var invBytes bytes.Buffer

	invBytes.WriteString("# [CHK_LATEST_VERSION]: ")
	invBytes.WriteString(inv.LatestVersion)
	invBytes.WriteString("\n---\n")

	type versionManifest struct {
		versions []string
		manifest []byte
	}

	var manifests []versionManifest
	for manifestHash, manifest := range inv.Manifests {
		var versions []string
		for version, hash := range inv.Versions {
			if hash == manifestHash {
				versions = append(versions, version)
			}
		}

		if len(versions) == 0 {
			continue
		}

		slices.SortFunc(versions, semver.Compare)

		manifests = append(manifests, versionManifest{
			versions: versions,
			manifest: manifest,
		})
	}

	slices.SortFunc(manifests, func(a, b versionManifest) int {
		return semver.Compare(a.versions[0], b.versions[0])
	})

	for _, manifest := range manifests {
		invBytes.WriteString("# [CHK_VERSIONS]: ")
		invBytes.WriteString(strings.Join(manifest.versions, ", "))
		invBytes.WriteString("\n")

		invBytes.Write(manifest.manifest)
		invBytes.WriteString("\n---\n")
	}

	if err := os.WriteFile(manifestsPath, invBytes.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write inventory file: %v", err)
	}

	return nil
}

func listVersions(ctx context.Context, maxVersion string) (map[string]struct{}, error) {
	result := bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--tags", "--sort=version:refname", "--refs", repoURL)
	cmd.Stdout = &result
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to list tags: %v", err)
	}

	// Parse the output of the git command
	lines := bytes.Split(result.Bytes(), []byte("\n"))

	versions := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		parts := bytes.Split(line, []byte("refs/tags/"))
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected output from git command: %s", line)
		}

		version := string(parts[1])

		// Skip any non-semver tags
		version = semver.Canonical(version)
		if version == "" {
			continue
		}

		// Skip any versions that are less than the min version
		if semver.Compare(version, minVersion) < 0 {
			continue
		}

		// Skip any versions that are greater than the max version
		if semver.Compare(version, maxVersion) > 0 {
			continue
		}

		versions[version] = struct{}{}
	}

	return versions, nil
}

func downloadManifests(ctx context.Context, version string) ([]byte, error) {
	url := fmt.Sprintf(downloadURL, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// cleanupManifests makes the manifests smaller, so they take up less space in the repo
// 1. Remove all comments from the yaml file
// 2. Remove all openapi CRD schemas
// 3. Keep only the CRD, Service and Deployment resources
func cleanupManifests(manifests []byte, version string) ([]byte, error) {
	resources := make([][]byte, 0)

	decoder := yaml.NewDecoder(bytes.NewBuffer(manifests))
	for {
		var manifest map[string]interface{}

		err := decoder.Decode(&manifest)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode manifest: %v", err)
		}
		if manifest == nil {
			continue
		}

		kind, ok := manifest["kind"].(string)
		if !ok {
			return nil, fmt.Errorf("kind is missing from manifest")
		}

		switch kind {
		case "CustomResourceDefinition":
			// remove all CRD schemas from yaml file
			switch spec := manifest["spec"].(type) {
			case map[string]interface{}:
				spec["versions"] = []interface{}{}
			case map[interface{}]interface{}:
				spec["versions"] = []interface{}{}
			}

			// remove status from CRD
			delete(manifest, "status")

		case "Service", "Deployment":
			// keep only the CRD, Service and Deployment resources from yaml file
		default:
			continue
		}

		yamlData, err := yaml.Marshal(manifest)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal manifest: %v", err)
		}

		resources = append(resources, yamlData)
	}

	manifests = bytes.Join(resources, []byte("\n---\n"))

	// Replace version with v99.99.99, this allows us to deduplicate the manifests
	// and reduce the size of the test_manifests.yaml file
	manifests = bytes.ReplaceAll(manifests, []byte(version), []byte(dummyVersion))

	for bytes.HasPrefix(manifests, []byte("\n")) || bytes.HasSuffix(manifests, []byte("\n---")) {
		manifests = bytes.TrimPrefix(manifests, []byte("\n"))
		manifests = bytes.TrimSuffix(manifests, []byte("\n---"))
	}

	for bytes.HasSuffix(manifests, []byte("\n")) || bytes.HasPrefix(manifests, []byte("\n---")) {
		manifests = bytes.TrimSuffix(manifests, []byte("\n"))
		manifests = bytes.TrimPrefix(manifests, []byte("\n---"))
	}

	return manifests, nil
}

func manifestHash(manifests []byte) (string, error) {
	hash := fnv.New64()
	if _, err := hash.Write(manifests); err != nil {
		return "", fmt.Errorf("failed to hash manifest: %v", err)
	}

	return hex.EncodeToString(hash.Sum([]byte{})), nil
}
