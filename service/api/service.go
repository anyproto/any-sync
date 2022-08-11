package api

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/document"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/treecache"
	"go.uber.org/zap"
	"io"
	"net/http"
	"time"
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
	cfg             *config.Config
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.treeCache = a.MustComponent(treecache.CName).(treecache.Service)
	s.documentService = a.MustComponent(document.CName).(document.Service)
	s.cfg = a.MustComponent(config.CName).(*config.Config)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	defer func() {
		if err == nil {
			log.With(zap.String("port", s.cfg.APIServer.Port)).Info("api server started running")
		}
	}()

	s.srv = &http.Server{
		Addr: fmt.Sprintf(":%s", s.cfg.APIServer.Port),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/treeDump", s.treeDump)
	mux.HandleFunc("/createDocumentTree", s.createDocumentTree)
	mux.HandleFunc("/createACLTree", s.createACLTree)
	mux.HandleFunc("/appendDocument", s.appendDocument)
	s.srv.Handler = mux

	go s.runServer()
	return nil
}

func (s *service) runServer() {
	err := s.srv.ListenAndServe()
	if err != nil {
		log.With(zap.Error(err)).Error("could not run api server")
	}
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
	err = s.treeCache.Do(context.Background(), treeId, func(obj interface{}) error {
		t := obj.(tree.CommonTree)
		dump, err = t.DebugDump()
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

func (s *service) createDocumentTree(w http.ResponseWriter, req *http.Request) {
	var (
		query     = req.URL.Query()
		text      = query.Get("text")
		aclTreeId = query.Get("aclTreeId")
	)
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	treeId, err := s.documentService.CreateDocumentTree(timeoutCtx, aclTreeId, text)
	cancel()
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, treeId)
}

func (s *service) createACLTree(w http.ResponseWriter, req *http.Request) {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	treeId, err := s.documentService.CreateACLTree(timeoutCtx)
	cancel()
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
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	err := s.documentService.UpdateDocumentTree(timeoutCtx, treeId, text)
	cancel()
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, fmt.Sprintf("updated document with id: %s with text: %s", treeId, text))
}

func sendText(r http.ResponseWriter, code int, body string) {
	r.Header().Set("Content-Type", "text/plain")
	r.WriteHeader(code)

	_, err := io.WriteString(r, fmt.Sprintf("%s\n", body))
	if err != nil {
		log.Error("writing response failed", zap.Error(err))
	}
}
