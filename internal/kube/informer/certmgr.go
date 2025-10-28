package informer

import (
	"context"
	"log/slog"

	"skupper-cert-manager/internal/kube/client"
	"skupper-cert-manager/internal/logger"

	cm "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	v1 "github.com/cert-manager/cert-manager/pkg/client/informers/externalversions/certmanager/v1"
	k8sv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func NewCertMgrCertificateInformer(cli *client.Client, namespace string) *CertMgrCertificateInformer {
	res := &CertMgrCertificateInformer{
		informer:     v1.NewCertificateInformer(cli.CertManager, namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		certificates: map[string]*cm.Certificate{},
		cli:          cli,
		logger:       logger.NewLogger("informer.cert-manager", namespace),
	}
	return res
}

type CertMgrCertificateInformer struct {
	informer     cache.SharedIndexInformer
	certificates map[string]*cm.Certificate
	logger       *slog.Logger
	cli          *client.Client
}

func (c *CertMgrCertificateInformer) Informer() cache.SharedIndexInformer {
	return c.informer
}

func (c *CertMgrCertificateInformer) Handle(key string) error {
	return Handle(key, c)
}

func (c *CertMgrCertificateInformer) Filter(obj *cm.Certificate) bool {
	return client.IsOwnedBySkupper(obj)
}

func (c *CertMgrCertificateInformer) Add(key string, obj *cm.Certificate) error {
	ready, reason := GetCertManagerCertificateReadyReason(obj)
	c.certificates[key] = obj
	certsCli := c.cli.Skupper.SkupperV2alpha1().Certificates(obj.Namespace)
	skupperCert, err := certsCli.Get(context.Background(), obj.Name, k8sv1.GetOptions{})
	if err != nil {
		c.logger.Error("Failed to get skupper certificate", "key", key, "error", err)
		return err
	}
	c.logger.Info("updating skupper certificate status", "key", key, "ready", ready, "reason", reason)
	err = SkupperCertificateReadyOrPending(c.cli, skupperCert, ready, reason)
	return err
}

func (c *CertMgrCertificateInformer) Update(key string, old, new *cm.Certificate) error {
	return c.Add(key, new)
}

func (c *CertMgrCertificateInformer) Delete(key string) error {
	delete(c.certificates, key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	certsCli := c.cli.Skupper.SkupperV2alpha1().Certificates(namespace)
	cert, err := certsCli.Get(context.Background(), name, k8sv1.GetOptions{})
	if err == nil {
		err = SkupperCertificateReadyOrPending(c.cli, cert, false, "Pending")
		if err != nil {
			c.logger.Error("Error updating certificate status to pending",
				"key", key, "error", err.Error())
			return err
		}
	}
	return nil
}

func (c *CertMgrCertificateInformer) Reconcile(key string, new *cm.Certificate) error {
	return nil
}

func (c *CertMgrCertificateInformer) Cache() map[string]*cm.Certificate {
	return c.certificates
}

func (c *CertMgrCertificateInformer) Equal(oldObj, newObj *cm.Certificate) bool {
	oldReady, oldReason := GetCertManagerCertificateReadyReason(oldObj)
	newReady, newReason := GetCertManagerCertificateReadyReason(newObj)
	return oldReady == newReady && oldReason == newReason
}

func GetCertManagerCertificateReadyReason(obj *cm.Certificate) (bool, string) {
	for _, condition := range obj.Status.Conditions {
		if condition.Type == cm.CertificateConditionReady {
			return condition.Status == metav1.ConditionTrue, condition.Reason
		}
	}
	return false, string(metav1.ConditionUnknown)
}
