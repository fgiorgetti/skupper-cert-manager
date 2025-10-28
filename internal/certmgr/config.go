package certmgr

import (
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

const (
	DefaultRootIssuerName = "skupper-issuer"
)

type Config struct {
	RootIssuer string
	Issuer     string
	IssuerMap  map[string]string
}

var (
	globalConfig    Config
	namespaceConfig map[string]Config
)

func init() {
	namespaceConfig = make(map[string]Config)
	//namespaceConfig["sk1"] = Config{
	//	IssuerMap: map[string]string{
	//		"skupper-site-ca": "custom-issuer",
	//	},
	//}
}

func GetRootIssuer(namespace string) (string, bool) {
	var rootIssuer string
	if nsConfig, ok := namespaceConfig[namespace]; ok {
		rootIssuer = nsConfig.RootIssuer
	}
	if rootIssuer == "" {
		rootIssuer = globalConfig.RootIssuer
	}
	clusterScope := false
	if len(rootIssuer) > 1 && rootIssuer[0] == '/' {
		rootIssuer = rootIssuer[1:]
		clusterScope = true
	}
	return rootIssuer, clusterScope
}

func GetIssuerFor(obj *v2alpha1.Certificate) (string, bool) {
	var issuer string
	var clusterScope bool
	ns := obj.Namespace
	if nsConfig, ok := namespaceConfig[ns]; ok {
		issuer = getIssuerFor(nsConfig, obj)
	}
	if issuer == "" {
		issuer = getIssuerFor(globalConfig, obj)
	}
	if len(issuer) > 1 && issuer[0] == '/' {
		issuer = issuer[1:]
		clusterScope = true
	}
	return issuer, clusterScope
}

func getIssuerFor(config Config, obj *v2alpha1.Certificate) string {
	ca := obj.Spec.Ca
	for from, to := range config.IssuerMap {
		if ca == from {
			return to
		}
	}
	if config.Issuer != "" {
		return config.Issuer
	}
	return ""
}
