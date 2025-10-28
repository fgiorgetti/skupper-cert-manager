package certmgr

import (
	"time"

	cm "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	v2 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DefaultExpiration() time.Duration {
	duration := time.Duration(5*365*24) * time.Hour
	return duration
}

func NewRootIssuer(namespace string) *cm.Issuer {
	var issuer = &cm.Issuer{
		TypeMeta: v1.TypeMeta{
			Kind:       "Issuer",
			APIVersion: cm.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      DefaultRootIssuerName,
			Namespace: namespace,
		},
		Spec: cm.IssuerSpec{
			IssuerConfig: cm.IssuerConfig{
				SelfSigned: &cm.SelfSignedIssuer{},
			},
		},
	}
	return issuer
}

func NewCACertificate(obj *v2alpha1.Certificate) *cm.Certificate {
	var issuer string
	var clusterIssuer bool
	issuer, clusterIssuer = GetIssuerFor(obj)
	if issuer == "" {
		issuer, clusterIssuer = GetRootIssuer(obj.Namespace)
	}
	var cmCert = &cm.Certificate{
		TypeMeta: v1.TypeMeta{
			Kind:       "Certificate",
			APIVersion: cm.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      obj.Name,
			Namespace: obj.Namespace,
			OwnerReferences: []v1.OwnerReference{
				*v1.NewControllerRef(obj, v2alpha1.SchemeGroupVersion.WithKind("Certificate")),
			},
		},
		Spec: cm.CertificateSpec{
			CommonName: obj.Spec.Subject,
			Duration: &v1.Duration{
				Duration: DefaultExpiration(),
			},
			DNSNames:   obj.Spec.Hosts,
			SecretName: obj.Name,
			IssuerRef: v2.ObjectReference{
				Name: valueOrDefault(issuer, DefaultRootIssuerName),
			},
			IsCA: true,
		},
	}
	if clusterIssuer {
		cmCert.Spec.IssuerRef.Kind = "ClusterIssuer"
	}
	return cmCert
}

func NewIssuer(obj *v2alpha1.Certificate) *cm.Issuer {
	var issuer = &cm.Issuer{
		TypeMeta: v1.TypeMeta{
			Kind:       "Issuer",
			APIVersion: cm.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      obj.Name,
			Namespace: obj.Namespace,
			OwnerReferences: []v1.OwnerReference{
				*v1.NewControllerRef(obj, v2alpha1.SchemeGroupVersion.WithKind("Certificate")),
			},
		},
		Spec: cm.IssuerSpec{
			IssuerConfig: cm.IssuerConfig{
				CA: &cm.CAIssuer{
					SecretName: obj.Name,
				},
			},
		},
	}
	return issuer
}

func NewCertificate(obj *v2alpha1.Certificate) *cm.Certificate {
	issuer, clusterIssuer := GetIssuerFor(obj)
	var cmCert = &cm.Certificate{
		TypeMeta: v1.TypeMeta{
			Kind:       "Certificate",
			APIVersion: cm.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      obj.Name,
			Namespace: obj.Namespace,
			OwnerReferences: []v1.OwnerReference{
				*v1.NewControllerRef(obj, v2alpha1.SchemeGroupVersion.WithKind("Certificate")),
			},
		},
		Spec: cm.CertificateSpec{
			CommonName: obj.Spec.Subject,
			Duration: &v1.Duration{
				Duration: DefaultExpiration(),
			},
			DNSNames:   obj.Spec.Hosts,
			SecretName: obj.Name,
			IssuerRef: v2.ObjectReference{
				Name: valueOrDefault(issuer, obj.Spec.Ca),
			},
		},
	}
	if clusterIssuer {
		cmCert.Spec.IssuerRef.Kind = "ClusterIssuer"
	}
	return cmCert
}

func valueOrDefault(value, dflt string) string {
	if value == "" {
		return dflt
	}
	return value
}
