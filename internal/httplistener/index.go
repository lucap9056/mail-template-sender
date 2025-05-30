package httplistener

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"

	"github.com/lucap9056/mail-template-sender/internal/smtp"

	"github.com/gin-gonic/gin"
	"github.com/lucap9056/mail-template-sender/httpclient"
	"github.com/lucap9056/mail-template-sender/internal/template"
)

type App struct {
	client         *smtp.SMTP
	templateGroups *template.TemplateGroups
	router         *gin.Engine
	ctx            context.Context
	cancel         context.CancelFunc
}

func New(client *smtp.SMTP, templateGroups *template.TemplateGroups) *App {

	router := gin.Default()

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		client:         client,
		templateGroups: templateGroups,
		router:         router,
		ctx:            ctx,
		cancel:         cancel,
	}

	router.POST("/", app.Handler)

	return app
}

func (app *App) Handler(c *gin.Context) {

	body := &httpclient.MailTemplateOptions[any]{}

	if err := c.BindJSON(body); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		log.Println(err.Error())
		return
	}

	msg, err := app.templateGroups.ToText(
		body.TemplateGroup,
		body.TemplateNames,
		app.client.Username(),
		body.Targets,
		body.Data,
	)

	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		log.Println("to text error: ", err.Error())
		return
	}

	if err := app.client.Send(body.Targets, msg); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		log.Println("smtp send error: ", err.Error())
		return
	}

	c.String(http.StatusOK, "")
}

func (app *App) Run(addr string, tlsConfig *tls.Config) error {

	server := &http.Server{
		Addr:      addr,
		Handler:   app.router,
		TLSConfig: tlsConfig,
	}

	if tlsConfig != nil {
		err := server.ListenAndServeTLS("", "")
		if err != nil {
			return err
		}
	} else {
		err := server.ListenAndServe()
		if err != nil {
			return err
		}
	}

	<-app.ctx.Done()
	return server.Close()
}

func (a *App) Stop() {
	a.cancel()
}
