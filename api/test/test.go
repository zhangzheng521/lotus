package test

import (
	"context"
	"testing"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/build"
	"github.com/stretchr/testify/assert"
)

type TestNode struct {
	api.FullNode

	MineOne func(context.Context) error
	Stop    func(context.Context) error
}

type TestStorageNode struct {
	api.StorageMiner
	Stop func(context.Context) error
}

type CleanupFunc func(context.Context)

// APIBuilder is a function which is invoked in test suite to provide
// test nodes and networks
//
// storage array defines storage nodes, numbers in the array specify full node
// index the storage node 'belongs' to
type APIBuilder func(t *testing.T, nFull int, storage []int) ([]TestNode, []TestStorageNode, CleanupFunc)
type testSuite struct {
	makeNodes APIBuilder
}

// TestApis is the entry point to API test suite
func TestApis(t *testing.T, b APIBuilder) {
	ts := testSuite{
		makeNodes: b,
	}

	t.Run("version", ts.testVersion)
	t.Run("id", ts.testID)
	t.Run("testConnectTwo", ts.testConnectTwo)
	t.Run("testMining", ts.testMining)
}

func (ts *testSuite) testVersion(t *testing.T) {
	ctx := context.Background()
	apis, _, cleanup := ts.makeNodes(t, 1, []int{})
	defer cleanup(ctx)
	api := apis[0]

	v, err := api.Version(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != build.Version {
		t.Error("Version didn't work properly")
	}
}

func (ts *testSuite) testID(t *testing.T) {
	ctx := context.Background()
	apis, _, cleanup := ts.makeNodes(t, 1, []int{})
	defer cleanup(ctx)
	api := apis[0]

	id, err := api.ID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.Regexp(t, "^12", id.Pretty())
}

func (ts *testSuite) testConnectTwo(t *testing.T) {
	ctx := context.Background()
	apis, _, cleanup := ts.makeNodes(t, 2, []int{})
	defer cleanup(ctx)

	p, err := apis[0].NetPeers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 0 {
		t.Error("Node 0 has a peer")
	}

	p, err = apis[1].NetPeers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 0 {
		t.Error("Node 1 has a peer")
	}

	addrs, err := apis[1].NetAddrsListen(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := apis[0].NetConnect(ctx, addrs); err != nil {
		t.Fatal(err)
	}

	p, err = apis[0].NetPeers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 1 {
		t.Error("Node 0 doesn't have 1 peer")
	}

	p, err = apis[1].NetPeers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 1 {
		t.Error("Node 0 doesn't have 1 peer")
	}
}
