package ceos

import (
	"fmt"
	topopb "github.com/google/kne/proto/topo"
	"github.com/google/kne/topo/node"
	scraplibase "github.com/scrapli/scrapligo/driver/base"
	scraplicore "github.com/scrapli/scrapligo/driver/core"
	scraplinetwork "github.com/scrapli/scrapligo/driver/network"
	scraplitest "github.com/scrapli/scrapligo/util/testhelper"
	"google.golang.org/protobuf/proto"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"testing"
	"time"
)

type nodeIntf struct {}

func (ni *nodeIntf) KubeClient() kubernetes.Interface {return nil}
func (ni *nodeIntf) RESTConfig() *rest.Config {return nil}
func (ni *nodeIntf) Interfaces() map[string]*node.Link {return nil}
func (ni *nodeIntf) Namespace() string {return "some-namespace"}

type testNode struct {
	Node
	pb *topopb.Node
	cliConn *scraplinetwork.Driver
}

func (n *testNode) SpawnCliConn(ni node.Interface, t *testing.T) error {

	fmt.Printf("n: %+v\n", n)
	fmt.Printf("n: %+v\n", n.pb.Name)

	d, err := scraplicore.NewCoreDriver(
		n.pb.Name,
		"arista_eos",
		scraplibase.WithAuthBypass(true),
		scraplibase.WithTimeoutOps(time.Second*30),
		// obv not a great path :) should modify the patched transport to not accept `t` and also to
		// have options for path or loaded string or something for the actual session data
		scraplitest.WithPatchedTransport("/Users/carl/dev/github/scrapligo/test_data/driver/network/sendcommand/arista_eos", t),
	)
	if err != nil {
		return err
	}

	// set kubectl exec command for scrapli transport -- requires embedding System in TestingTransport...
	transport, _ := d.Transport.(*scraplitest.TestingTransport)
	transport.ExecCmd = "kubectl"
	transport.OpenCmd = []string{"exec", "-it", "-n", ni.Namespace(), n.pb.Name, "--", "Cli"}

	n.cliConn = d

	return nil
}

func testNew(pb *topopb.Node) (node.Implementation, error) {
	cfg := defaults(pb)
	proto.Merge(cfg, pb)
	node.FixServices(cfg)
	return &testNode{
		pb: cfg,
	}, nil
}

func TestGenerateSelfSigned(t *testing.T) {
	bn := &topopb.Node{
		Name:        "testEosNode",
		Type:        2,
		Config:      &topopb.Config{
			Cert:    &topopb.CertificateCfg{
				Config: &topopb.CertificateCfg_SelfSigned{
					SelfSigned: &topopb.SelfSignedCertCfg{
						CertName: "my_cert",
						KeyName: "my_key",
						KeySize: 1024,
					},
				},
			},
		},
	}

	n, err := testNew(bn)

	if err != nil {
		t.Fatalf("failed creating kne arista node")
	}

	fmt.Printf("N: %+v\n", n)

	typedN, _ := n.(*testNode)

	ni := &nodeIntf{}
	_ = typedN.SpawnCliConn(ni, t)

	typedTransport, _ := typedN.cliConn.Transport.(*scraplitest.TestingTransport)
	fmt.Printf("OpenCmd: %+v\n", typedTransport.OpenCmd)

	// obv can assert the open cmd / exec cmd get set correctly

	// in theory once we have pointed the testing transport at legit session data this should
	// just "work" for testing loading up certs and stuff
}
