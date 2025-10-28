package informer

import (
	"context"
	"log/slog"
	"reflect"

	"skupper-cert-manager/internal/certmgr"
	"skupper-cert-manager/internal/kube/client"
	"skupper-cert-manager/internal/logger"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	informerv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/informers/externalversions/skupper/v2alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	controllerKey  = "certificate-controller"
	controllerName = "cert-manager"
)

func NewSkupperCertificateInformer(cli *client.Client, namespace string) *SkupperCertificateInformer {
	res := &SkupperCertificateInformer{
		informer:     informerv2alpha1.NewCertificateInformer(cli.Skupper, namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
		certificates: map[string]*v2alpha1.Certificate{},
		cli:          cli,
		logger:       logger.NewLogger("informer.skupper", namespace),
	}
	return res
}

type SkupperCertificateInformer struct {
	informer     cache.SharedIndexInformer
	certificates map[string]*v2alpha1.Certificate
	logger       *slog.Logger
	cli          *client.Client
}

func (c *SkupperCertificateInformer) Handle(key string) error {
	return Handle(key, c)
}

func (c *SkupperCertificateInformer) Filter(obj *v2alpha1.Certificate) bool {
	if name, ok := obj.Spec.Settings[controllerKey]; ok {
		return name == controllerName
	}
	return false
}

func (c *SkupperCertificateInformer) Add(key string, obj *v2alpha1.Certificate) error {
	return c.Reconcile(key, obj)
}

func (c *SkupperCertificateInformer) Delete(key string) error {
	_, ok := c.certificates[key]
	if !ok {
		return nil
	}
	c.logger.Info("Certificate has been deleted", "key", key)
	delete(c.certificates, key)
	return nil
}

func (c *SkupperCertificateInformer) Update(key string, old, new *v2alpha1.Certificate) error {
	return c.Reconcile(key, new)
}

func (c *SkupperCertificateInformer) Reconcile(key string, obj *v2alpha1.Certificate) error {
	var err error
	if err = c.createRootIssuer(obj.Namespace); err != nil {
		return err
	}
	if obj.Spec.Signing {
		if err = c.ensureCACert(key, obj); err != nil {
			return err
		}
		return c.ensureIssuerFor(obj)
	}
	if err = c.ensureNoIssuerFor(obj); err != nil {
		return err
	}
	return c.createCertificateFor(key, obj)
}

func (c *SkupperCertificateInformer) Cache() map[string]*v2alpha1.Certificate {
	return c.certificates
}

func (c *SkupperCertificateInformer) Equal(olbObj, newObj *v2alpha1.Certificate) bool {
	return reflect.DeepEqual(olbObj.Spec, newObj.Spec)
}

func (c *SkupperCertificateInformer) Spec(obj *v2alpha1.Certificate) v2alpha1.CertificateSpec {
	return obj.Spec
}

func (c *SkupperCertificateInformer) Informer() cache.SharedIndexInformer {
	return c.informer
}

func (c *SkupperCertificateInformer) ensureCACert(key string, obj *v2alpha1.Certificate) error {
	var err error
	if currentCert, ok := c.certificates[key]; ok {
		if reflect.DeepEqual(obj.Spec, currentCert.Spec) {
			return nil
		}
	}
	caCert := certmgr.NewCACertificate(obj)
	certsCli := c.cli.CertManager.CertmanagerV1().Certificates(obj.Namespace)
	c.logger.Debug("Loading cert-manager CA certificate", "key", key)
	currentCmCaCert, err := certsCli.Get(context.Background(), obj.Name, v1.GetOptions{})
	if err == nil {
		if !reflect.DeepEqual(currentCmCaCert.Spec, caCert.Spec) {
			c.logger.Info("Updating existing CA certificate", "key", key)
			currentCmCaCert.Spec = caCert.Spec
			_, err = certsCli.Update(context.Background(), currentCmCaCert, v1.UpdateOptions{})
			if err != nil {
				c.logger.Error("Failed to update existing CA certificate", "key", key, "error", err)
				return err
			}
			c.logger.Info("Updated CA certificate", "key", key)
			c.certificates[key] = obj
		}
	} else {
		c.logger.Info("Creating cert-manager CA certificate", "key", key)
		_, err = certsCli.Create(context.Background(), caCert, v1.CreateOptions{})
		if err != nil {
			c.logger.Error("Failed to create cert-manager CA certificate", "key", key, "error", err)
			return err
		}
		c.certificates[key] = obj
		if err = SkupperCertificateReadyOrPending(c.cli, obj, false, "Pending"); err != nil {
			c.logger.Error("Failed to set CA certificate as configured", "key", key, "error", err)
			return err
		}
	}
	return nil
}

func (c *SkupperCertificateInformer) ensureIssuerFor(obj *v2alpha1.Certificate) error {
	key, _ := cache.MetaNamespaceKeyFunc(obj)
	issuersCli := c.cli.CertManager.CertmanagerV1().Issuers(obj.Namespace)
	_, err := issuersCli.Get(context.Background(), obj.Name, v1.GetOptions{})
	if err == nil {
		c.logger.Debug("Issuer already exists", "key", key)
		return nil
	}
	issuer := certmgr.NewIssuer(obj)
	c.logger.Info("Creating Issuer", "key", key)
	_, err = issuersCli.Create(context.Background(), issuer, v1.CreateOptions{})
	if err != nil {
		c.logger.Error("Failed to create issuer", "key", key, "error", err)
	}
	return err
}

func (c *SkupperCertificateInformer) createRootIssuer(namespace string) error {
	if !c.needsRootIssuer(namespace) {
		c.logger.Debug("Skipping root issuer creation", "target-namespace", namespace)
		return nil
	}
	rootIssuer := certmgr.NewRootIssuer(namespace)
	issuersCli := c.cli.CertManager.CertmanagerV1().Issuers(namespace)
	_, err := issuersCli.Get(context.Background(), certmgr.DefaultRootIssuerName, v1.GetOptions{})
	if err == nil {
		c.logger.Debug("Root Issuer already exists", "target-namespace", namespace, "name", certmgr.DefaultRootIssuerName)
		return nil
	}
	c.logger.Info("Creating Root Issuer", "target-namespace", namespace, "name", certmgr.DefaultRootIssuerName)
	_, err = issuersCli.Create(context.Background(), rootIssuer, v1.CreateOptions{})
	if err != nil {
		c.logger.Error("Failed to create root issuer", "target-namespace", namespace, "name", certmgr.DefaultRootIssuerName, "error", err)
		return err
	}
	return nil
}

func (c *SkupperCertificateInformer) createCertificateFor(key string, obj *v2alpha1.Certificate) error {
	if currentCert, ok := c.certificates[key]; ok {
		if reflect.DeepEqual(obj.Spec, currentCert.Spec) {
			return nil
		}
	}
	desired := certmgr.NewCertificate(obj)
	certsCli := c.cli.CertManager.CertmanagerV1().Certificates(obj.Namespace)
	current, err := certsCli.Get(context.Background(), obj.Name, v1.GetOptions{})
	if err == nil {
		c.logger.Debug("Certificate already exists", "key", key)
		if !reflect.DeepEqual(current.Spec, desired.Spec) {
			c.logger.Debug("Updating existing certificate", "key", key)
			current.Spec = desired.Spec
			_, err = certsCli.Update(context.Background(), current, v1.UpdateOptions{})
			if err != nil {
				c.logger.Error("Failed to update existing certificate", "key", key, "error", err)
				return err
			}
			c.certificates[key] = obj
		}
		return nil
	}
	c.logger.Info("Creating Certificate", "key", key)
	_, err = certsCli.Create(context.Background(), desired, v1.CreateOptions{})
	if err != nil {
		c.logger.Error("Failed to create certificate", "key", key, "error", err)
	}
	c.certificates[key] = obj
	if err = SkupperCertificateReadyOrPending(c.cli, obj, false, "Pending"); err != nil {
		c.logger.Error("Failed to set certificate as configured", "key", key, "error", err)
		return err
	}
	return err
}

func (c *SkupperCertificateInformer) needsRootIssuer(namespace string) bool {
	rootIssuer, _ := certmgr.GetRootIssuer(namespace)
	return rootIssuer == ""
}

func (c *SkupperCertificateInformer) ensureNoIssuerFor(obj *v2alpha1.Certificate) error {
	issuersCli := c.cli.CertManager.CertmanagerV1().Issuers(obj.Namespace)
	issuer, err := issuersCli.Get(context.Background(), obj.Name, v1.GetOptions{})
	if err == nil {
		if client.IsOwnedBy(issuer, obj, v2alpha1.SchemeGroupVersion.WithKind("Certificate")) {
			c.logger.Info("Removing issuer no longer needed", "target-namespace", obj.Namespace, "target-name", obj.Name)
			err = issuersCli.Delete(context.Background(), obj.Name, v1.DeleteOptions{})
			return err
		}
	}
	return nil
}

func SkupperCertificateReadyOrPending(cli *client.Client, obj *v2alpha1.Certificate, ready bool, message string) error {
	condition := v2alpha1.ReadyCondition()
	if !ready {
		condition = v2alpha1.PendingCondition(message)
	}
	certsCli := cli.Skupper.SkupperV2alpha1().Certificates(obj.Namespace)
	obj.Status.SetCondition(v2alpha1.CONDITION_TYPE_READY, condition, obj.Generation)
	_, err := certsCli.UpdateStatus(context.Background(), obj, v1.UpdateOptions{})
	return err
}
