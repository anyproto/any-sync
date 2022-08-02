package api

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/acltree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/document"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/treecache"
	"go.uber.org/zap"
	"io"
	"net/http"
)

const CName = "APIService"

var log = logger.NewNamed("api")

func New() app.Component {
	return &service{}
}

type service struct {
	treeCache       treecache.Service
	documentService document.Service
	srv             *http.Server
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.treeCache = a.MustComponent(treecache.CName).(treecache.Service)
	s.documentService = a.MustComponent(document.CName).(document.Service)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	s.srv = &http.Server{
		Addr: ":8080",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/treeDump", s.treeDump)
	mux.HandleFunc("/createDocument", s.createDocument)
	mux.HandleFunc("/appendDocument", s.appendDocument)
	s.srv.Handler = mux

	return s.srv.ListenAndServe()
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.srv.Shutdown(ctx)
}

func (s *service) treeDump(w http.ResponseWriter, req *http.Request) {
	var (
		query  = req.URL.Query()
		treeId = query.Get("treeId")
		dump   string
		err    error
	)
	err = s.treeCache.Do(context.Background(), treeId, func(tree acltree.ACLTree) error {
		dump, err = tree.DebugDump()
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, dump)
}

func (s *service) createDocument(w http.ResponseWriter, req *http.Request) {
	var (
		query = req.URL.Query()
		text  = query.Get("text")
	)
	treeId, err := s.documentService.CreateDocument(context.Background(), text)
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, treeId)
}

func (s *service) appendDocument(w http.ResponseWriter, req *http.Request) {
	var (
		query  = req.URL.Query()
		text   = query.Get("text")
		treeId = query.Get("treeId")
	)
	err := s.documentService.UpdateDocument(context.Background(), treeId, text)
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, "")
}

func sendText(r http.ResponseWriter, code int, body string) {
	r.Header().Set("Content-Type", "text/plain")
	r.WriteHeader(code)

	_, err := io.WriteString(r, fmt.Sprintf("%s\n", body))
	if err != nil {
		log.Error("writing response failed", zap.Error(err))
	}
}
