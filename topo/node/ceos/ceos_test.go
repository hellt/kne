package ceos

import (
	"context"
	"testing"

	topopb "github.com/google/kne/proto/topo"
	"github.com/google/kne/topo/node"
	scraplibase "github.com/scrapli/scrapligo/driver/base"
	scraplicore "github.com/scrapli/scrapligo/driver/core"
	scraplinetwork "github.com/scrapli/scrapligo/driver/network"
	scraplitest "github.com/scrapli/scrapligo/util/testhelper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	ktest "k8s.io/client-go/testing"
)

type fakeNode struct {
	kClient    kubernetes.Interface
	namespace  string
	interfaces map[string]*node.Link
	rCfg       *rest.Config
}

func (f *fakeNode) KubeClient() kubernetes.Interface {
	return f.kClient
}

func (f *fakeNode) RESTConfig() *rest.Config {
	return f.rCfg
}

func (f *fakeNode) Interfaces() map[string]*node.Link {
	return f.interfaces
}

func (f *fakeNode) Namespace() string {
	return f.namespace
}

type fakeWatch struct {
	e []watch.Event
}

func (f *fakeWatch) Stop() {}

func (f *fakeWatch) ResultChan() <-chan watch.Event {
	eCh := make(chan watch.Event)
	go func() {
		for len(f.e) != 0 {
			e := f.e[0]
			f.e = f.e[1:]
			eCh <- e
		}
	}()
	return eCh
}

func TestGenerateSelfSigned(t *testing.T) {
	ki := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod1",
		},
	})

	reaction := func(action ktest.Action) (handled bool, ret watch.Interface, err error) {
		f := &fakeWatch{
			e: []watch.Event{{
				Object: &corev1.Pod{
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			}},
		}
		return true, f, nil
	}
	ki.PrependWatchReactor("*", reaction)

	ni := &fakeNode{
		kClient:   ki,
		namespace: "test",
	}

	bn := &topopb.Node{
		Name: "testEosNode",
		Type: 2,
		Config: &topopb.Config{
			Cert: &topopb.CertificateCfg{
				Config: &topopb.CertificateCfg_SelfSigned{
					SelfSigned: &topopb.SelfSignedCertCfg{
						CertName: "my_cert",
						KeyName:  "my_key",
						KeySize:  2048,
					},
				},
			},
		},
	}

	n, err := New(bn)

	if err != nil {
		t.Fatalf("failed creating kne arista node")
	}

	nImpl, _ := n.(*Node)

	oldNewCoreDriver := scraplicore.NewCoreDriver
	defer func() { scraplicore.NewCoreDriver = oldNewCoreDriver }()
	scraplicore.NewCoreDriver = func(host, platform string, options ...scraplibase.Option) (*scraplinetwork.Driver, error) {
		return scraplicore.NewEOSDriver(
			host,
			scraplibase.WithAuthBypass(true),
			scraplitest.WithPatchedTransport("generate_certificate_success"),
		)
	}

	ctx := context.Background()

	err = nImpl.GenerateSelfSigned(ctx, ni)
	if err != nil {
		t.Fatalf("generating self signed cert failed, error: %+v\n", err)
	}
}
