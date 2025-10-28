package main

import (
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"skupper-cert-manager/internal/kube/client"
	"skupper-cert-manager/internal/kube/informer"
)

/*
TODO - Add ownerRefs to all created resources

TODO - Tasks breakdown

  - Draft a workflow for processing certificates
  - Must have:
    - Default Root Issuer (used to sign the CA certificates)
    - Default Issuer (used to sign Client and Server certificates)
    - Issuer mapping (based on certificates.skupper.io.spec.ca)
  - Create a kubernetes client type in a separate package
  - Test it outside and inside the cluster
  - Create a rate limited queue processor
  - Define scope (namespace or cluster)
  - Test it to make sure it is watching for resources properly

# TODO - Implementation notes

  - When a certificate, which was delegated to the certificate-manager is no longer delegated,
    I believe we have to:
  - delete the cert-manager Certificate, Issuer resources
  - delete the corresponding Secrets
*/

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	stopCh := make(chan struct{})
	cli, err := client.NewClient("", "")
	if err != nil {
		log.Fatal(err)
	}
	skpCertInformer := informer.NewSkupperCertificateInformer(cli, "")
	cmCertInformer := informer.NewCertMgrCertificateInformer(cli, "")
	eventProcessor := client.NewEventProcessor("")
	var informerErrors []error
	for _, i := range []client.EventInformer{skpCertInformer, cmCertInformer} {
		informerErrors = append(informerErrors, eventProcessor.AddInformer(i))
	}
	if errors.Join(informerErrors...) != nil {
		log.Fatal(err)
	}
	eventProcessor.StartInformers(stopCh)
	eventProcessor.Start(stopCh)
	<-sigs
}
