package server

import (
	"fmt"
	"net/http"
	"strings"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gin-gonic/gin"
)

type BFS struct {
	FileSystem http.FileSystem
	Root       string
}

func (bfs *BFS) Open(name string) (http.File, error) {
	return bfs.FileSystem.Open(name)
}

func (bfs *BFS) Exists(prefix string, filepath string) bool {
	var err error
	var url string
	url = strings.TrimPrefix(filepath, prefix)
	if len(url) < len(filepath) {
		_, err = bfs.FileSystem.Open(url)
		if err != nil {
			return false
		}
		return true
	}
	return false
}

func GetBFS(root string) *BFS {
	fs := &assetfs.AssetFS{
		Asset:     Asset,
		AssetDir:  AssetDir,
		AssetInfo: AssetInfo,
		Prefix:    root,
		Fallback:  "",
	}

	return &BFS{fs, root}
}

func (bfs *BFS) Collection(c *gin.Context) {
	result := Result{}
	result.Code = 200
	result.Result = "OK"
	var err error
	var collection = c.Params.ByName("collection")
	result.Message = make([]string, 0)
	result.Message, err = AssetDir(fmt.Sprintf("%s/img/%s", bfs.Root, collection))
	if err != nil {
		result.Code = 404
		result.Result = "Error"
		result.Message = err
	}
	c.JSON(result.Code, result)
}
