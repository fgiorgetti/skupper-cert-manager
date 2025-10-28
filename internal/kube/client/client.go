package client

import (
	"reflect"

	cmclientset "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	skclientset "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	CertManager *cmclientset.Clientset
	Skupper     *skclientset.Clientset
	Kube        *kubernetes.Clientset
}

func NewClient(kubeContext, kubeConfig string) (*Client, error) {
	c := new(Client)

	loader := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeConfig != "" {
		loader.ExplicitPath = kubeConfig
	}
	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: kubeContext,
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, overrides).ClientConfig()
	if err != nil {
		return nil, err
	}

	// Clients: cert-manager, skupper and core
	cm, err := cmclientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	sk, err := skclientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	c.CertManager = cm
	c.Skupper = sk
	c.Kube = k8s
	return c, nil
}

func IsOwnedBy(ownedObj, ownerObj v1.Object, gvk schema.GroupVersionKind) bool {
	current := v1.GetControllerOf(ownedObj)
	expected := v1.NewControllerRef(ownerObj, gvk)
	return reflect.DeepEqual(current, expected)
}

func IsOwnedBySkupper(ownedObj v1.Object) bool {
	owner := v1.GetControllerOf(ownedObj)
	return owner != nil && owner.APIVersion == "skupper.io/v2alpha1"
}
