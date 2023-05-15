package main

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"golang.org/x/exp/slog"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	awsRegion := os.Getenv("AWS_REGION")
	endpoint := os.Getenv("WSS_MANAGEMENT_ENDPOINT")
	log.Info("Starting up", slog.String("region", awsRegion), slog.String("endpoint", endpoint))
	h := NewHandler(log, awsRegion, endpoint)
	lambda.Start(h.Handle)
}

func NewHandler(log *slog.Logger, awsRegion, endpoint string) Handler {
	return Handler{
		Log:       log,
		AWSRegion: awsRegion,
		Endpoint:  endpoint,
	}
}

type Handler struct {
	Log       *slog.Logger
	AWSRegion string
	Endpoint  string
}

func (h Handler) Handle(ctx context.Context, req events.SQSEvent) (resp events.SQSEventResponse, err error) {
	h.Log.Info("Received message")

	// Create API client.
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(h.AWSRegion))
	if err != nil {
		return
	}
	api := apigatewaymanagementapi.NewFromConfig(cfg, apigatewaymanagementapi.WithEndpointResolver(apigatewaymanagementapi.EndpointResolverFromURL(h.Endpoint)))

	// Leave 5 seconds at the end to clean up.
	deadline, _ := ctx.Deadline()
	deadline = deadline.Add(5 * time.Second)

	// Track processed message IDs.
	messageIDToIsProcessed := make(map[string]bool)
	for _, rec := range req.Records {
		messageIDToIsProcessed[rec.MessageId] = false
	}

	// Process messages.
	for _, rec := range req.Records {
		connectionIdAttr, hasConnectionIDAttr := rec.MessageAttributes["connectionId"]
		if !hasConnectionIDAttr {
			h.Log.Warn("skipping sending, no connection ID attribute found", slog.String("messageId", rec.MessageId))
			continue
		}
		connectionID := connectionIdAttr.StringValue
		if connectionID == nil {
			h.Log.Warn("skipping sending, no connection ID found", slog.String("messageId", rec.MessageId))
			continue
		}
		//TODO: Support sending to topics by looking up in the DB.
		_, err := api.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: connectionID,
			Data:         []byte(rec.Body),
		})
		if err != nil {
			h.Log.Warn("failed to post to web socket connection, failing message", slog.String("messageId", rec.MessageId), slog.String("connectionId", *connectionID), slog.Any("error", err))
			continue
		}
		messageIDToIsProcessed[rec.MessageId] = true
		// Break if there's only a few seconds remaining.
		if time.Now().After(deadline) {
			break
		}
	}

	// Retry failed messages.
	for messageID, isProcessed := range messageIDToIsProcessed {
		if isProcessed {
			continue
		}
		resp.BatchItemFailures = append(resp.BatchItemFailures, events.SQSBatchItemFailure{
			ItemIdentifier: messageID,
		})
	}

	return
}
