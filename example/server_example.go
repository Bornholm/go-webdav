package example

import (
	"net/http"

	"github.com/bornholm/go-webdav/filesystem/local"
	"github.com/bornholm/go-webdav/handler"
)

func ExampleServer() {
	fs := local.NewFileSystem("my/dir")
	handler := handler.New(fs)

	if err := http.ListenAndServe(":8080", handler); err != nil {
		panic(err)
	}
}
