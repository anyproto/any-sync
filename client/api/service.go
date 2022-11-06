package api

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/document"
	clientstorage "github.com/anytypeio/go-anytype-infrastructure-experiments/client/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
)

const CName = "api.service"

var log = logger.NewNamed("api")

func New() Service {
	return &service{}
}

type Service interface {
	app.ComponentRunnable
}

type service struct {
	controller Controller
	srv        *http.Server
	cfg        *config.Config
}

func (s *service) Init(a *app.App) (err error) {
	s.controller = newController(
		a.MustComponent(clientspace.CName).(clientspace.Service),
		a.MustComponent(storage.CName).(clientstorage.ClientStorage),
		a.MustComponent(document.CName).(document.Service),
		a.MustComponent(account.CName).(account.Service))
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
	mux.HandleFunc("/deriveSpace", s.deriveSpace)
	mux.HandleFunc("/createSpace", s.createSpace)
	mux.HandleFunc("/loadSpace", s.loadSpace)
	mux.HandleFunc("/allSpaceIds", s.allSpaceIds)
	mux.HandleFunc("/createDocument", s.createDocument)
	mux.HandleFunc("/allDocumentIds", s.allDocumentIds)
	mux.HandleFunc("/addText", s.addText)
	mux.HandleFunc("/dumpDocumentTree", s.dumpDocumentTree)
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

func (s *service) deriveSpace(w http.ResponseWriter, req *http.Request) {
	id, err := s.controller.DeriveSpace()
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, id)
}

func (s *service) loadSpace(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	spaceId := query.Get("spaceId")
	err := s.controller.LoadSpace(query.Get("spaceId"))
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, spaceId)
}

func (s *service) createSpace(w http.ResponseWriter, req *http.Request) {
	id, err := s.controller.CreateSpace()
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, id)
}

func (s *service) allSpaceIds(w http.ResponseWriter, req *http.Request) {
	ids, err := s.controller.AllSpaceIds()
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, strings.Join(ids, "\n"))
}

func (s *service) createDocument(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	spaceId := query.Get("spaceId")
	id, err := s.controller.CreateDocument(spaceId)
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, id)
}

func (s *service) allDocumentIds(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	spaceId := query.Get("spaceId")
	ids, err := s.controller.AllDocumentIds(spaceId)
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, strings.Join(ids, "\n"))
}

func (s *service) addText(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	spaceId := query.Get("spaceId")
	documentId := query.Get("documentId")
	text := query.Get("text")
	err := s.controller.AddText(spaceId, documentId, text)
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, "Text added")
}

func (s *service) dumpDocumentTree(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	spaceId := query.Get("spaceId")
	documentId := query.Get("documentId")
	dump, err := s.controller.DumpDocumentTree(spaceId, documentId)
	if err != nil {
		sendText(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendText(w, http.StatusOK, dump)
}

func sendText(r http.ResponseWriter, code int, body string) {
	r.Header().Set("Content-Type", "text/plain")
	r.WriteHeader(code)

	_, err := io.WriteString(r, fmt.Sprintf("%s\n", body))
	if err != nil {
		log.Error("writing response failed", zap.Error(err))
	}
}
