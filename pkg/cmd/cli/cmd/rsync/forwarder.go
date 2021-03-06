package rsync

import (
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/portforward"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// portForwarder starts port forwarding to a given pod
type portForwarder struct {
	Namespace string
	PodName   string
	Client    *kclient.Client
	Config    *restclient.Config
}

// ensure that portForwarder implements the forwarder interface
var _ forwarder = &portForwarder{}

// ForwardPorts will forward a set of ports from a pod, the stopChan will stop the forwarding
// when it's closed or receives a struct{}
func (f *portForwarder) ForwardPorts(ports []string, stopChan <-chan struct{}) error {
	req := f.Client.RESTClient.Post().
		Resource("pods").
		Namespace(f.Namespace).
		Name(f.PodName).
		SubResource("portforward")

	dialer, err := remotecommand.NewExecutor(f.Config, "POST", req.URL())
	if err != nil {
		return err
	}
	fw, err := portforward.New(dialer, ports, stopChan)
	if err != nil {
		return err
	}
	ready := make(chan struct{})
	errChan := make(chan error)
	fw.Ready = ready
	go func() { errChan <- fw.ForwardPorts() }()
	select {
	case <-ready:
		return nil
	case err = <-errChan:
		return err
	}
}

// newPortForwarder creates a new forwarder for use with rsync
func newPortForwarder(f *clientcmd.Factory, o *RsyncOptions) (forwarder, error) {
	client, err := f.Client()
	if err != nil {
		return nil, err
	}
	config, err := f.ClientConfig()
	if err != nil {
		return nil, err
	}
	return &portForwarder{
		Namespace: o.Namespace,
		PodName:   o.PodName(),
		Client:    client,
		Config:    config,
	}, nil
}
