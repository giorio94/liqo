package tenantControlNamespace

import (
	"context"
	"fmt"
	"strings"

	"github.com/liqotech/liqo/pkg/discovery"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type tenantControlNamespaceManager struct {
	client kubernetes.Interface
}

// create a new TenantControlNamespaceManager object
func NewTenantControlNamespaceManager(client kubernetes.Interface) TenantControlNamespaceManager {
	return &tenantControlNamespaceManager{
		client: client,
	}
}

// create a new Tenant Control Namespace given the clusterID
// This method is idempotent, multiple calls of it will not lead to multiple namespace creations
func (nm *tenantControlNamespaceManager) CreateNamespace(clusterID string) (*v1.Namespace, error) {
	// first check that it does not exist yet
	ns, err := nm.GetNamespace(clusterID)
	if err == nil {
		return ns, nil
	} else if !kerrors.IsNotFound(err) {
		// an error occurred, but it is not a not found error
		klog.Error(err)
		return nil, err
	}
	// a not found error occurred, create the namespace

	ns = &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: strings.Join([]string{tenantControlNamespaceRoot, ""}, "-"),
			Labels: map[string]string{
				discovery.ClusterIdLabel:              clusterID,
				discovery.TenantControlNamespaceLabel: "true",
			},
		},
	}
	return nm.client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
}

// get a Tenant Control Namespace given the clusterID
func (nm *tenantControlNamespaceManager) GetNamespace(clusterID string) (*v1.Namespace, error) {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			discovery.ClusterIdLabel:              clusterID,
			discovery.TenantControlNamespaceLabel: "true",
		},
	}

	namespaces, err := nm.client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	})
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	if nItems := len(namespaces.Items); nItems == 0 {
		err = kerrors.NewNotFound(v1.Resource("Namespace"), clusterID)
		klog.Error(err)
		return nil, err
	} else if nItems > 1 {
		err = fmt.Errorf("multiple tenant control namespaces found for clusterID %v", clusterID)
		klog.Error(err)
		return nil, err
	}
	return &namespaces.Items[0], nil
}