/*
Copyright 2020 The cert-manager Authors.

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

package convert

import (
	corev1 "k8s.io/api/core/v1"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"

	internalacmeinstall "github.com/cert-manager/cmctl/v2/pkg/convert/internal/apis/acme/install"
	internalcertmanagerinstall "github.com/cert-manager/cmctl/v2/pkg/convert/internal/apis/certmanager/install"
	internalmetainstall "github.com/cert-manager/cmctl/v2/pkg/convert/internal/apis/meta/install"
)

var Scheme = func() *runtime.Scheme {
	scheme := runtime.NewScheme()
	internalacmeinstall.Install(scheme)
	internalcertmanagerinstall.Install(scheme)
	internalmetainstall.Install(scheme)

	// This is used to add the List object type
	listGroupVersion := schema.GroupVersionKind{Group: "", Version: runtime.APIVersionInternal, Kind: "List"}
	scheme.AddKnownTypeWithName(listGroupVersion, &metainternalversion.List{})
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})

	utilruntime.Must(kscheme.AddToScheme(scheme))
	utilruntime.Must(metainternalversion.AddToScheme(scheme))

	// Adds the conversion between internalmeta.List and corev1.List
	_ = scheme.AddConversionFunc((*corev1.List)(nil), (*metainternalversion.List)(nil), func(a, b interface{}, scope conversion.Scope) error {
		metaList := &metav1.List{}
		metaList.Items = a.(*corev1.List).Items
		return metainternalversion.Convert_v1_List_To_internalversion_List(metaList, b.(*metainternalversion.List), scope)
	})

	_ = scheme.AddConversionFunc((*metainternalversion.List)(nil), (*corev1.List)(nil), func(a, b interface{}, scope conversion.Scope) error {
		metaList := &metav1.List{}
		err := metainternalversion.Convert_internalversion_List_To_v1_List(a.(*metainternalversion.List), metaList, scope)
		if err != nil {
			return err
		}
		b.(*corev1.List).Items = metaList.Items
		return nil
	})

	return scheme
}()
