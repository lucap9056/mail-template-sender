package grpcclient

import (
	"context"
	"encoding/json"
	"time"

	"github.com/lucap9056/mail-template-sender/grpcstruct"
	"google.golang.org/grpc"
)

type Client struct {
	client grpcstruct.MailTemplateClient
	conn   *grpc.ClientConn
}

func New(target string, opts ...grpc.DialOption) (*Client, error) {

	conn, err := grpc.NewClient(target, opts...)

	if err != nil {
		return nil, err
	}

	client := &Client{
		grpcstruct.NewMailTemplateClient(conn),
		conn,
	}

	return client, nil
}

type MailTemplateOptions[T any] struct {
	TemplateGroup string
	TemplateNames []string
	Targets       []string
	Data          T
}

func (c *Client) Send(ctx context.Context, options *MailTemplateOptions[any]) error {

	dataJson, err := json.Marshal(options.Data)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	req := &grpcstruct.MailTemplateRequest{
		TemplateGroup: options.TemplateGroup,
		TemplateNames: options.TemplateNames,
		To:            options.Targets,
		DataJson:      dataJson,
	}

	_, err = c.client.Send(ctx, req)

	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}
