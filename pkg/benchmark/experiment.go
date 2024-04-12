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

package benchmark

import (
	"context"
	"fmt"
	"time"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	logf "github.com/cert-manager/cert-manager/pkg/logs"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type phase struct {
	name     string
	disabled bool
	f        func(context.Context) error
}

type experiment struct {
	options
	measurements *measurements
}

func (o *experiment) run(ctx context.Context) error {
	logger := logf.FromContext(ctx, "benchmark")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		t := time.NewTicker(o.measurementInterval)
		defer t.Stop()
		for {
			if err := o.measurements.new(ctx); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return nil
			case <-t.C:
			}
		}

	})

	phases := []phase{
		{
			name: "ramp-up",
			f: func(ctx context.Context) error {
				t := time.NewTicker(o.rampUpLoadInterval)
				defer t.Stop()
				for {
					remaining := o.rampUpTargetCertificateCount - o.measurements.latest().CertificateCount
					if remaining <= 0 {
						return nil
					}
					if err := o.load(ctx); err != nil {
						return err
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-t.C:
					}
				}
			},
		},
		{
			name: "catch-up",
			f: func(ctx context.Context) error {
				t := time.NewTicker(time.Second)
				defer t.Stop()
				for {
					r := o.measurements.latest()
					if r.CertificateRequestCount == r.CertificateCount {
						return nil
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-t.C:
					}
				}
			},
		},
		{
			name: "steady-state",
			f: func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(o.steadyStateDuration):
				}
				return nil
			},
		},
		{
			name:     "cleanup",
			disabled: o.cleanupDisabled,
			f: func(ctx context.Context) error {
				t := time.NewTicker(time.Second)
				defer t.Stop()
				for {
					r := o.measurements.latest()
					if r.CertificateCount == 0 {
						return nil
					}

					nsList, err := o.KubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
						Limit:         10,
						LabelSelector: fmt.Sprintf("%s=true", label),
					})
					if err != nil {
						return err
					}
					for _, ns := range nsList.Items {
						if err := o.KubeClient.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{}); err != nil {
							if !errors.IsNotFound(err) {
								logger.Error(err, "While deleting namespace", "namespace", ns.Name)
							}
						}
					}

					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-t.C:
					}
				}
			},
		},
		{
			name: "final-measurements",
			f: func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(o.finalMeasurementsDuration):
				}
				return nil
			},
		},
	}

	g.Go(func() error {
		for _, phase := range phases {
			if phase.disabled {
				logger.Info("skipping-phase", "name", phase.name)
				continue
			}
			logger.Info("new-phase", "name", phase.name)
			if err := phase.f(ctx); err != nil {
				return err
			}
		}
		cancel()
		return nil
	})

	return g.Wait()
}

func (o *experiment) load(ctx context.Context) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "team-",
			Labels: map[string]string{
				label: "true",
			},
		},
	}

	ns, err := o.KubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	issuer := &cmapi.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ns.Name,
			Namespace: ns.Name,
			Labels: map[string]string{
				label: "true",
			},
		},
		Spec: cmapi.IssuerSpec{
			IssuerConfig: cmapi.IssuerConfig{
				SelfSigned: &cmapi.SelfSignedIssuer{},
			},
		},
	}

	_, err = o.CMClient.CertmanagerV1().Issuers(issuer.Namespace).Create(ctx, issuer, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	g, gCTX := errgroup.WithContext(ctx)
	g.SetLimit(10)
	for i := 0; i < 10; i++ {
		secretName := fmt.Sprintf("app-%d", i)

		certificate := &cmapi.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: issuer.Namespace,
				Labels: map[string]string{
					label: "true",
				},
			},
			Spec: cmapi.CertificateSpec{
				CommonName: secretName,
				SecretName: secretName,
				PrivateKey: &cmapi.CertificatePrivateKey{
					Algorithm:      cmapi.PrivateKeyAlgorithm(o.rampUpCertificateAlgorithm),
					Size:           o.rampUpCertificateSize,
					RotationPolicy: cmapi.RotationPolicyAlways,
				},
				IssuerRef: cmmeta.ObjectReference{
					Name: issuer.Name,
				},
			},
		}
		g.Go(func() error {
			_, err = o.CMClient.CertmanagerV1().Certificates(certificate.Namespace).Create(gCTX, certificate, metav1.CreateOptions{})
			return err
		})
	}
	return g.Wait()

}