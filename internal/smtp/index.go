package smtp

import (
	"crypto/tls"
	"net"
	"net/smtp"
)

type SMTPConfig struct {
	Username string
	Password string
	Address  string
}

type SMTP struct {
	cfg    *SMTPConfig
	conn   *tls.Conn
	client *smtp.Client
}

func New(cfg *SMTPConfig) (*SMTP, error) {

	host, _, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		ServerName: host,
	}

	conn, err := tls.Dial("tcp", cfg.Address, tlsConfig)
	if err != nil {
		return nil, err
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return nil, err
	}

	err = client.Noop()
	if err != nil {
		return nil, err
	}

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, host)
	err = client.Auth(auth)
	if err != nil {
		return nil, err
	}

	return &SMTP{cfg, conn, client}, nil
}

func (s *SMTP) Username() string {
	return s.cfg.Username
}

func (s *SMTP) Send(to []string, msg []byte) error {

	err := s.client.Reset()
	if err != nil {
		return err
	}

	err = s.client.Mail(s.cfg.Username)
	if err != nil {
		return err
	}

	for _, target := range to {
		err := s.client.Rcpt(target)
		if err != nil {
			return err
		}

	}

	wc, err := s.client.Data()
	if err != nil {
		return err
	}
	defer wc.Close()

	_, err = wc.Write(msg)
	if err != nil {
		return err
	}

	return nil
}

func (s *SMTP) Close() {
	s.conn.Close()
	s.client.Close()
}
