package client

import (
	"context"

	topolsv1 "github.com/kvaster/topols/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kind = "LogicalVolume"
	//kindList = "LogicalVolumeList"
)

type wrappedReader struct {
	client client.Reader
	scheme *runtime.Scheme
}

var _ client.Reader = &wrappedReader{}

func NewWrappedReader(c client.Reader, s *runtime.Scheme) client.Reader {
	return &wrappedReader{
		client: c,
		scheme: s,
	}
}

func (c *wrappedReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	switch obj.(type) {
	case *unstructured.Unstructured:
		return c.client.Get(ctx, key, obj, opts...)
	case *metav1.PartialObjectMetadata:
		return c.client.Get(ctx, key, obj, opts...)
	case *topolsv1.LogicalVolume:
		return c.client.Get(ctx, key, obj, opts...)
	}
	return c.client.Get(ctx, key, obj, opts...)
}

func (c *wrappedReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	switch list.(type) {
	case *unstructured.UnstructuredList:
		return c.client.List(ctx, list, opts...)
	case *metav1.PartialObjectMetadataList:
		return c.client.List(ctx, list, opts...)
	case *topolsv1.LogicalVolumeList:
		return c.client.List(ctx, list, opts...)
	}
	return c.client.List(ctx, list, opts...)
}

type wrappedClient struct {
	reader client.Reader
	client client.Client
}

var _ client.Client = &wrappedClient{}

func NewWrappedClient(c client.Client) client.Client {
	return &wrappedClient{
		reader: NewWrappedReader(c, c.Scheme()),
		client: c,
	}
}

func (c *wrappedClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.reader.Get(ctx, key, obj, opts...)
}

func (c *wrappedClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.reader.List(ctx, list, opts...)
}

func (c *wrappedClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	switch obj.(type) {
	case *unstructured.Unstructured:
		return c.client.Create(ctx, obj, opts...)
	case *metav1.PartialObjectMetadata:
		return c.client.Create(ctx, obj, opts...)
	case *topolsv1.LogicalVolume:
		return c.client.Create(ctx, obj, opts...)
	}
	return c.client.Create(ctx, obj, opts...)
}

func (c *wrappedClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	switch obj.(type) {
	case *unstructured.Unstructured:
		return c.client.Delete(ctx, obj, opts...)
	case *metav1.PartialObjectMetadata:
		return c.client.Delete(ctx, obj, opts...)
	case *topolsv1.LogicalVolume:
		return c.client.Delete(ctx, obj, opts...)
	}
	return c.client.Delete(ctx, obj, opts...)
}

func (c *wrappedClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	switch obj.(type) {
	case *unstructured.Unstructured:
		return c.client.Update(ctx, obj, opts...)
	case *metav1.PartialObjectMetadata:
		return c.client.Update(ctx, obj, opts...)
	case *topolsv1.LogicalVolume:
		return c.client.Update(ctx, obj, opts...)
	}
	return c.client.Update(ctx, obj, opts...)
}

// wrappedClient assumes that LogicalVolume definitions on topolvm.io and topolvm.cybozu.com are identical.
// Since patch processes resources as Objects, even if the structs are different, if the Spec and Status are the same, there is no problem with patch processing.
// ref: https://github.com/kubernetes-sigs/controller-runtime/blob/v0.12.1/pkg/client/patch.go#L114
func (c *wrappedClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	switch obj.(type) {
	case *unstructured.Unstructured:
		return c.client.Patch(ctx, obj, patch, opts...)
	case *metav1.PartialObjectMetadata:
	case *topolsv1.LogicalVolume:
		return c.client.Patch(ctx, obj, patch, opts...)
	}
	return c.client.Patch(ctx, obj, patch, opts...)
}

func (c *wrappedClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	switch obj.(type) {
	case *unstructured.Unstructured:
		return c.client.DeleteAllOf(ctx, obj, opts...)
	case *metav1.PartialObjectMetadata:
		return c.client.DeleteAllOf(ctx, obj, opts...)
	case *topolsv1.LogicalVolume:
		return c.client.DeleteAllOf(ctx, obj, opts...)
	}
	return c.client.DeleteAllOf(ctx, obj, opts...)
}

func (c *wrappedClient) Status() client.StatusWriter {
	return &wrappedStatusWriter{client: c.client}
}

func (c *wrappedClient) Scheme() *runtime.Scheme {
	return c.client.Scheme()
}

func (c *wrappedClient) RESTMapper() meta.RESTMapper {
	return c.client.RESTMapper()
}

type wrappedStatusWriter struct {
	client client.Client
}

var _ client.StatusWriter = &wrappedStatusWriter{}

func (c *wrappedStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	switch obj.(type) {
	case *unstructured.Unstructured:
		return c.client.Status().Update(ctx, obj, opts...)
	case *metav1.PartialObjectMetadata:
		return c.client.Status().Update(ctx, obj, opts...)
	case *topolsv1.LogicalVolume:
		return c.client.Status().Update(ctx, obj, opts...)
	}
	return c.client.Status().Update(ctx, obj, opts...)
}

// wrappedClient assumes that LogicalVolume definitions on topolvm.io and topolvm.cybozu.com are identical.
// Since patch processes resources as Objects, even if the structs are different, if the Spec and Status are the same, there is no problem with patch processing.
// ref: https://github.com/kubernetes-sigs/controller-runtime/blob/v0.12.1/pkg/client/patch.go#L114
func (c *wrappedStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	switch obj.(type) {
	case *unstructured.Unstructured:
		return c.client.Status().Patch(ctx, obj, patch, opts...)
	case *metav1.PartialObjectMetadata:
		return c.client.Status().Patch(ctx, obj, patch, opts...)
	case *topolsv1.LogicalVolume:
		return c.client.Status().Patch(ctx, obj, patch, opts...)
	}
	return c.client.Status().Patch(ctx, obj, patch, opts...)
}
