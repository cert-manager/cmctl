package benchmark

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	logf "github.com/cert-manager/cert-manager/pkg/logs"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsapi "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

type record struct {
	Time                    int64 `json:"timestamp"`
	CertificateCount        int64 `json:"certificate_count"`
	CertificateRequestCount int64 `json:"certificaterequest_count"`
	SecretCount             int64 `json:"secret_count"`
	SecretSize              int64 `json:"secret_size"`
	ControllerMemory        int64 `json:"controller_memory"`
	ControllerCPU           int64 `json:"controller_cpu"`
	WebhookMemory           int64 `json:"webhook_memory"`
	WebhookCPU              int64 `json:"webhook_cpu"`
	CAInjectorMemory        int64 `json:"cainjector_memory"`
	CAInjectorCPU           int64 `json:"cainjector_cpu"`
}

type measurements struct {
	sync.RWMutex
	options
	record
	encoder *json.Encoder
}

func (o *measurements) certificateCount(ctx context.Context) error {
	l, err := o.CMClient.CertmanagerV1().Certificates("").List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return err
	}
	if l.RemainingItemCount != nil {
		o.CertificateCount = *l.RemainingItemCount + 1
	}
	return nil
}

func (o *measurements) certificateRequestCount(ctx context.Context) error {
	l, err := o.CMClient.CertmanagerV1().CertificateRequests("").List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return err
	}
	if l.RemainingItemCount != nil {
		o.CertificateRequestCount = *l.RemainingItemCount + 1
	}
	return nil
}

func (o *measurements) secretCount(ctx context.Context) error {
	l, err := o.KubeClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return err
	}
	if l.RemainingItemCount != nil {
		o.SecretCount = *l.RemainingItemCount + 1
	}
	return nil
}

func (o *measurements) secretSize(ctx context.Context) error {
	l, err := o.KubeClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	var size int64
	for _, s := range l.Items {
		for _, d := range s.Data {
			size += int64(len(d))
		}
	}
	o.SecretSize = size
	return nil
}

func (o *measurements) certManagerResources(ctx context.Context) error {
	c, err := o.RESTClientGetter.ToDiscoveryClient()
	if err != nil {
		return err
	}
	res := c.RESTClient().
		Get().
		RequestURI("/apis/metrics.k8s.io/v1beta1/namespaces/cert-manager/pods").
		Do(ctx)
	if err := res.Error(); err != nil {
		return err
	}
	var m metricsapi.PodMetricsList
	if err := res.Into(&m); err != nil {
		return err
	}
	for _, i := range m.Items {
		cpu := i.Containers[0].Usage.Cpu().MilliValue()
		memory := i.Containers[0].Usage.Memory().MilliValue()
		switch i.Labels["app.kubernetes.io/component"] {
		case "controller":
			o.record.ControllerCPU = cpu
			o.record.ControllerMemory = memory
		case "webhook":
			o.record.WebhookCPU = cpu
			o.record.WebhookMemory = memory
		case "cainjector":
			o.record.CAInjectorCPU = cpu
			o.record.CAInjectorMemory = memory
		}
	}
	return nil
}

func (o *measurements) latest() record {
	o.RLock()
	defer o.RUnlock()
	return o.record
}

func newMeasurements(options options) *measurements {
	return &measurements{
		options: options,
		encoder: json.NewEncoder(options.Out),
	}
}

func (o *measurements) new(ctx context.Context) error {
	logger := logf.FromContext(ctx, "benchmark")

	o.Lock()
	defer o.Unlock()
	o.record = record{}
	g, gCTX := errgroup.WithContext(ctx)
	for _, f := range []func(context.Context) error{
		o.certificateCount,
		o.certificateRequestCount,
		o.secretCount,
		o.secretSize,
		o.certManagerResources,
	} {
		f := f
		g.Go(func() error {
			return f(gCTX)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	o.record.Time = time.Now().UTC().Unix()
	logger.V(logf.DebugLevel).Info("measurement", "data", o.record)
	if err := o.encoder.Encode(o.record); err != nil {
		return err
	}
	return nil
}
