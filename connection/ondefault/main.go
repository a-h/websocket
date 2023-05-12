package main

import (
	"context"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"golang.org/x/exp/slog"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := NewHandler(log)
	lambda.Start(h.Handle)
}

func NewHandler(log *slog.Logger) Handler {
	return Handler{
		Log: log,
	}
}

type Handler struct {
	Log *slog.Logger
}

func (h Handler) Handle(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (resp events.APIGatewayProxyResponse) {
	h.Log.Info("Received default request", slog.String("connectionId", req.RequestContext.ConnectionID))
	resp.StatusCode = 200
	resp.Body = `{ "ok": true }`
	return
}
