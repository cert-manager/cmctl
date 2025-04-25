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

package versionchecker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/cert-manager/cmctl/v2/internal/versionchecker"

	_ "embed"
)

const dummyVersion = "v99.99.99"

type testManifest struct {
	versions []string
	manifest []byte
}

func loadManifests(t *testing.T) []testManifest {
	testManifestBytes, err := os.ReadFile("testdata/test_manifests.yaml")
	if err != nil {
		t.Fatal(err)
	}

	// Read latest version from first line
	split := bytes.SplitN(testManifestBytes, []byte("\n"), 2)
	if len(split) != 2 {
		t.Fatal(fmt.Errorf("invalid test manifest: %s", testManifestBytes))
	}

	latestVersion := strings.TrimSpace(string(split[0]))
	latestVersion = strings.TrimPrefix(latestVersion, "# [CHK_LATEST_VERSION]: ")
	if latestVersion == "" {
		t.Fatal(fmt.Errorf("invalid test manifest: %s", testManifestBytes))
	}

	t.Log("Latest version:", latestVersion)

	testManifestBytes = split[1]

	var manifests []testManifest
	for _, manifest := range bytes.Split(testManifestBytes, []byte("---\n# [CHK_VERSIONS]: ")) {
		if len(manifest) == 0 {
			continue
		}

		parts := bytes.SplitN(manifest, []byte("\n"), 2)
		if len(parts) != 2 {
			t.Fatal(fmt.Errorf("invalid test manifest: %s", manifest))
		}

		versions := string(parts[0])
		versions = strings.TrimSpace(versions)

		manifests = append(manifests, testManifest{
			versions: strings.Split(versions, ", "),
			manifest: parts[1],
		})
	}

	return manifests
}

func manifestToObject(manifest io.Reader) ([]runtime.Object, error) {
	obj, err := resource.
		NewLocalBuilder().
		Flatten().
		Unstructured().
		Stream(manifest, "").
		Do().
		Object()
	if err != nil {
		return nil, err
	}

	list, ok := obj.(*corev1.List)
	if !ok {
		return nil, errors.New("Could not get list")
	}

	return transformObjects(list.Items)
}

func transformObjects(objects []runtime.RawExtension) ([]runtime.Object, error) {
	transformedObjects := []runtime.Object{}
	for _, resource := range objects {
		var err error
		gvk := resource.Object.GetObjectKind().GroupVersionKind()

		// Create a pod for a Deployment resource
		if gvk.Group == "apps" && gvk.Version == "v1" && gvk.Kind == "Deployment" {
			unstr := resource.Object.(*unstructured.Unstructured)

			var deployment appsv1.Deployment
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.Object, &deployment)
			if err != nil {
				return nil, err
			}

			pod, err := getPodFromTemplate(&deployment.Spec.Template, resource.Object, nil)
			if err != nil {
				return nil, err
			}

			transformedObjects = append(transformedObjects, pod)
		}

		transformedObjects = append(transformedObjects, resource.Object)
	}

	return transformedObjects, nil
}

func setupFakeVersionChecker(manifest io.Reader) (*versionchecker.VersionChecker, error) {
	scheme := runtime.NewScheme()

	if err := kscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := apiextensionsv1beta1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	objs, err := manifestToObject(manifest)
	if err != nil {
		return nil, err
	}

	cl := fake.
		NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()

	return versionchecker.NewFromClient(cl), nil
}

func TestVersionChecker(t *testing.T) {
	for _, item := range loadManifests(t) {
		for _, version := range item.versions {
			if version == "v1.2.0-alpha.1" {
				// Skip this version as it has a known issue: the CRDs are double
				continue
			}

			manifest := item.manifest
			manifest = bytes.ReplaceAll(manifest, []byte(dummyVersion), []byte(version))

			t.Run(version, func(t *testing.T) {
				checker, err := setupFakeVersionChecker(bytes.NewReader(manifest))
				if err != nil {
					t.Fatal(err)
				}

				versionGuess, err := checker.Version(t.Context())
				if err != nil {
					t.Fatalf("failed to detect expected version %s: %s", version, err)
				}

				if version != versionGuess.Detected {
					t.Fatalf("wrong -> expected: %s vs detected: %s", version, versionGuess)
				}
			})
		}
	}
}
