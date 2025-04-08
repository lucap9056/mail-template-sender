package smtp

import (
	"net/smtp"
)

type SMTPConfig struct {
	Username string
	Password string
	Address  string
	Host     string
	Port     string
}

type Client struct {
	cfg  *SMTPConfig
	auth smtp.Auth
}

func New(cfg *SMTPConfig) (*Client, error) {

	return &Client{
		cfg:  cfg,
		auth: smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host),
	}, nil
}

func (client *Client) Username() string {
	return client.cfg.Username
}

func (client *Client) Host() string {
	return client.cfg.Host
}

func (client *Client) Send(to []string, msg []byte) error {

	auth := client.auth
	addr := client.cfg.Address
	from := client.cfg.Username

	return smtp.SendMail(addr, auth, from, to, msg)
}
