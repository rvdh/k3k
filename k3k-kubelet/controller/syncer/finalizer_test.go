package syncer

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/k3k/k3k-kubelet/translate"
	"github.com/rancher/k3k/pkg/apis/k3k.io/v1beta1"
)

// A virtual object being deleted whose host copy is already gone (e.g. removed by an
// earlier reconcile whose finalizer update failed) must still lose its finalizer.
func TestFinalizerRemovedWhenHostObjectIsGone(t *testing.T) {
	now := metav1.Now()
	deleting := metav1.ObjectMeta{
		Name:              "web",
		Namespace:         virtualNamespace,
		DeletionTimestamp: &now,
	}

	tests := []struct {
		name      string
		object    client.Object
		reconcile func(*Context, reconcile.Request) (reconcile.Result, error)
	}{
		{
			name:   "service",
			object: &corev1.Service{ObjectMeta: withFinalizer(deleting, serviceFinalizerName)},
			reconcile: func(c *Context, req reconcile.Request) (reconcile.Result, error) {
				return (&ServiceReconciler{Context: c}).Reconcile(t.Context(), req)
			},
		},
		{
			name:   "ingress",
			object: &networkingv1.Ingress{ObjectMeta: withFinalizer(deleting, ingressFinalizerName)},
			reconcile: func(c *Context, req reconcile.Request) (reconcile.Result, error) {
				return (&IngressReconciler{Context: c}).Reconcile(t.Context(), req)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, networkingv1.AddToScheme(scheme))
			require.NoError(t, v1beta1.AddToScheme(scheme))

			ctx := &Context{
				VirtualClient:    fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.object).Build(),
				HostClient:       fake.NewClientBuilder().WithScheme(scheme).WithObjects(newTestCluster()).Build(),
				Translator:       translate.ToHostTranslator{ClusterName: testClusterName, ClusterNamespace: testClusterNamespace},
				ClusterName:      testClusterName,
				ClusterNamespace: testClusterNamespace,
			}
			key := types.NamespacedName{Name: "web", Namespace: virtualNamespace}

			_, err := tt.reconcile(ctx, reconcile.Request{NamespacedName: key})
			require.NoError(t, err)

			err = ctx.VirtualClient.Get(t.Context(), key, tt.object)
			require.True(t, apierrors.IsNotFound(err), "virtual object should be gone, got %v", err)
		})
	}
}

func withFinalizer(meta metav1.ObjectMeta, finalizer string) metav1.ObjectMeta {
	meta.Finalizers = []string{finalizer}
	return meta
}
