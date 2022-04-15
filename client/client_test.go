package client

import (
	"bytes"
	"context"
	"embed"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	types "github.com/filecoin-project/venus/venus-shared/types/market/client"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-merkledag"
	unixfile "github.com/ipfs/go-unixfs/file"
	"github.com/ipld/go-car"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus-market/v2/imports"
	"github.com/filecoin-project/venus-market/v2/storageprovider"
)

//go:embed testdata/*
var testdata embed.FS

func TestImportLocal(t *testing.T) {
	ctx := context.Background()
	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	dir := t.TempDir()
	im := imports.NewManager(ctx, ds, dir)

	a := &API{
		Imports:                   im,
		StorageBlockstoreAccessor: storageprovider.NewImportsBlockstoreAccessor(im),
	}

	b, err := testdata.ReadFile("testdata/payload.txt")
	require.NoError(t, err)

	root, err := a.ClientImportLocal(ctx, bytes.NewReader(b))
	require.NoError(t, err)
	require.NotEqual(t, cid.Undef, root)

	list, err := a.ClientListImports(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	it := list[0]
	require.Equal(t, root, *it.Root)
	require.True(t, strings.HasPrefix(it.CARPath, dir))

	local, err := a.ClientHasLocal(ctx, root)
	require.NoError(t, err)
	require.True(t, local)

	order := types.ExportRef{
		Root:         root,
		FromLocalCAR: it.CARPath,
	}

	// retrieve as UnixFS.
	out1 := filepath.Join(dir, "retrieval1.data") // as unixfs
	out2 := filepath.Join(dir, "retrieval2.data") // as car
	err = a.ClientExport(ctx, order, types.FileRef{
		Path: out1,
	})
	require.NoError(t, err)

	outBytes, err := ioutil.ReadFile(out1)
	require.NoError(t, err)
	require.Equal(t, b, outBytes)

	err = a.ClientExport(ctx, order, types.FileRef{
		Path:  out2,
		IsCAR: true,
	})
	require.NoError(t, err)

	// open the CARv2 being custodied by the import manager
	orig, err := carv2.OpenReader(it.CARPath)
	require.NoError(t, err)

	// open the CARv1 we just exported
	exported, err := carv2.OpenReader(out2)
	require.NoError(t, err)

	require.EqualValues(t, 1, exported.Version)
	require.EqualValues(t, 2, orig.Version)

	origRoots, err := orig.Roots()
	require.NoError(t, err)
	require.Len(t, origRoots, 1)

	exportedRoots, err := exported.Roots()
	require.NoError(t, err)
	require.Len(t, exportedRoots, 1)

	require.EqualValues(t, origRoots, exportedRoots)

	// recreate the unixfs dag, and see if it matches the original file byte by byte
	// import the car into a memory blockstore, then export the unixfs file.
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	_, err = car.LoadCar(ctx, bs, exported.DataReader())
	require.NoError(t, err)

	dag := merkledag.NewDAGService(blockservice.New(bs, offline.Exchange(bs)))

	nd, err := dag.Get(ctx, exportedRoots[0])
	require.NoError(t, err)

	file, err := unixfile.NewUnixfsFile(ctx, dag, nd)
	require.NoError(t, err)

	exportedPath := filepath.Join(dir, "exported.data")
	err = files.WriteTo(file, exportedPath)
	require.NoError(t, err)

	exportedBytes, err := ioutil.ReadFile(exportedPath)
	require.NoError(t, err)

	// compare original file to recreated unixfs file.
	require.Equal(t, b, exportedBytes)
}

func TestGetPortFromAddr(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{
			"common",
			"http://127.0.0.1:41231",
			"41231",
			false,
		},
		{
			"http",
			"/ip4/127.0.0.1/tcp/41231/http",
			"41231",
			false,
		},
		{
			"ws",
			"/ip4/127.0.0.1/tcp/41231/ws",
			"41231",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getPortFromAddr(tt.addr)

			if err != nil != tt.wantErr {
				t.Fatal(err)
			}

			if got != tt.want {
				t.Fatal(err)
			}
		})
	}

}
