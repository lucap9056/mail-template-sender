package grpclistener

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"

	"github.com/lucap9056/mail-template-sender/grpcstruct"
	"github.com/lucap9056/mail-template-sender/internal/smtp"
	"github.com/lucap9056/mail-template-sender/internal/template"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type App struct {
	grpcstruct.UnimplementedMailTemplateServer
	server         *grpc.Server
	client         *smtp.Client
	templateGroups *template.TemplateGroups
	ctx            context.Context
	cancel         context.CancelFunc
}

func New(client *smtp.Client, templateGroups *template.TemplateGroups, tlsConfig *tls.Config) (*App, error) {

	var server *grpc.Server

	if tlsConfig != nil {
		creds := credentials.NewTLS(tlsConfig)
		server = grpc.NewServer(grpc.Creds(creds))

	} else {
		server = grpc.NewServer()
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		server:         server,
		client:         client,
		templateGroups: templateGroups,
		ctx:            ctx,
		cancel:         cancel,
	}

	grpcstruct.RegisterMailTemplateServer(server, app)

	return app, nil
}

func (app *App) Send(ctx context.Context, req *grpcstruct.MailTemplateRequest) (*grpcstruct.MailTemplateResponse, error) {

	res := &grpcstruct.MailTemplateResponse{
		Ok: false,
	}

	var data any

	err := json.Unmarshal(req.DataJson, &data)
	if err != nil {
		return res, err
	}

	msg, err := app.templateGroups.ToText(req.TemplateGroup, req.TemplateName, app.client.Username(), req.To, data)
	if err != nil {
		return res, err
	}

	if err := app.client.Send(req.To, msg); err != nil {
		return res, err
	}

	res.Ok = true

	return res, nil
}

func (a *App) Run(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	err = a.server.Serve(listener)
	if err != nil {
		return err
	}
	<-a.ctx.Done()
	listener.Close()
	return nil
}

func (a *App) Stop() {
	a.cancel()
}
