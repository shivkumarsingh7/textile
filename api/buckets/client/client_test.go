package client_test

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/apitest"
	c "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	hc "github.com/textileio/textile/api/hub/client"
	"google.golang.org/grpc"
)

func TestClient_ListPath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	file, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file.Close()
	_, file1Root, err := client.PushPath(ctx, "mybuck1/file1.jpg", file)
	require.Nil(t, err)
	_, _, err = client.PushPath(ctx, "mybuck2/file1.jpg", file)
	require.Nil(t, err)

	t.Run("buckets", func(t *testing.T) {
		rep, err := client.ListPath(ctx, "")
		require.Nil(t, err)
		assert.Equal(t, len(rep.Item.Items), 2)
	})

	t.Run("bucket path", func(t *testing.T) {
		rep, err := client.ListPath(ctx, "mybuck1/file1.jpg")
		require.Nil(t, err)
		assert.True(t, strings.HasSuffix(rep.Item.Path, "file1.jpg"))
		assert.False(t, rep.Item.IsDir)
		assert.Equal(t, rep.Root.Path, file1Root.String())
	})
}

func TestClient_PushPath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	t.Run("bucket path", func(t *testing.T) {
		file, err := os.Open("testdata/file1.jpg")
		require.Nil(t, err)
		defer file.Close()
		progress := make(chan int64)
		go func() {
			for p := range progress {
				t.Logf("progress: %d", p)
			}
		}()
		pth, root, err := client.PushPath(ctx, "mybuck/file1.jpg", file, c.WithProgress(progress))
		require.Nil(t, err)
		assert.NotEmpty(t, pth)
		assert.NotEmpty(t, root)
	})

	t.Run("nested bucket path", func(t *testing.T) {
		file, err := os.Open("testdata/file2.jpg")
		require.Nil(t, err)
		defer file.Close()
		progress := make(chan int64)
		go func() {
			for p := range progress {
				t.Logf("progress: %d", p)
			}
		}()
		pth, root, err := client.PushPath(ctx, "mybuck/path/to/file2.jpg", file, c.WithProgress(progress))
		require.Nil(t, err)
		assert.NotEmpty(t, pth)
		assert.NotEmpty(t, root)

		rep, err := client.ListPath(ctx, "mybuck")
		require.Nil(t, err)
		assert.Equal(t, len(rep.Item.Items), 2)
	})
}

func TestClient_PullPath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	file, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file.Close()
	_, _, err = client.PushPath(ctx, "mybuck/file1.jpg", file)
	require.Nil(t, err)

	t.Run("bucket path", func(t *testing.T) {
		file, err := ioutil.TempFile("", "")
		require.Nil(t, err)
		defer file.Close()

		progress := make(chan int64)
		go func() {
			for p := range progress {
				t.Logf("progress: %d", p)
			}
		}()
		err = client.PullPath(ctx, "mybuck/file1.jpg", file, c.WithProgress(progress))
		require.Nil(t, err)
		info, err := file.Stat()
		require.Nil(t, err)
		t.Logf("wrote file with size %d", info.Size())
	})
}

func TestClient_RemovePath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	file1, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.Nil(t, err)
	defer file2.Close()
	_, _, err = client.PushPath(ctx, "mybuck1/file1.jpg", file1)
	require.Nil(t, err)
	_, _, err = client.PushPath(ctx, "mybuck1/again/file2.jpg", file1)
	require.Nil(t, err)

	t.Run("bucket path", func(t *testing.T) {
		err := client.RemovePath(ctx, "mybuck1/again/file2.jpg")
		require.Nil(t, err)
		_, err = client.ListPath(ctx, "mybuck1/again/file2.jpg")
		require.NotNil(t, err)
		_, err = client.ListPath(ctx, "mybuck1")
		require.Nil(t, err)
	})

	_, _, err = client.PushPath(ctx, "mybuck2/file1.jpg", file1)
	require.Nil(t, err)

	t.Run("entire bucket", func(t *testing.T) {
		err := client.RemovePath(ctx, "mybuck2/file1.jpg")
		require.Nil(t, err)
		_, err = client.ListPath(ctx, "mybuck2")
		require.NotNil(t, err)
	})
}

func TestClose(t *testing.T) {
	t.Parallel()
	conf, shutdown := apitest.MakeTextile(t)
	defer shutdown()
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	require.Nil(t, err)
	client, err := c.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{}))
	require.Nil(t, err)

	t.Run("test close", func(t *testing.T) {
		err := client.Close()
		require.Nil(t, err)
	})
}

func setup(t *testing.T) (context.Context, *c.Client, func()) {
	conf, shutdown := apitest.MakeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	require.Nil(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	client, err := c.NewClient(target, opts...)
	require.Nil(t, err)
	hubclient, err := hc.NewClient(target, opts...)
	require.Nil(t, err)
	threadsclient, err := tc.NewClient(target, opts...)
	require.Nil(t, err)

	user := apitest.Signup(t, hubclient, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	id := thread.NewIDV1(thread.Raw, 32)
	ctx = common.NewThreadNameContext(ctx, "buckets")
	err = threadsclient.NewDB(ctx, id)
	require.Nil(t, err)
	ctx = common.NewThreadIDContext(ctx, id)

	return ctx, client, func() {
		shutdown()
		err := client.Close()
		require.Nil(t, err)
	}
}