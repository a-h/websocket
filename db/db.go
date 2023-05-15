package db

import (
	"context"
	"fmt"
	"time"

	"github.com/a-h/websocket/backoff"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type StoreOption func(*StoreOptions) error

type StoreOptions struct {
	Region string
	Client *dynamodb.Client
}

func WithRegion(region string) StoreOption {
	return func(o *StoreOptions) error {
		o.Region = region
		return nil
	}
}

func WithClient(client *dynamodb.Client) StoreOption {
	return func(o *StoreOptions) error {
		o.Client = client
		return nil
	}
}

// NewStore creates a new store using default config.
func NewStore(ctx context.Context, tableName, namespace string, opts ...StoreOption) (s *Store, err error) {
	o := StoreOptions{}
	for _, opt := range opts {
		err = opt(&o)
		if err != nil {
			return
		}
	}
	if o.Client == nil {
		var cfg aws.Config
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(o.Region))
		if err != nil {
			return
		}
		o.Client = dynamodb.NewFromConfig(cfg)
	}
	s = &Store{
		Client:    o.Client,
		TableName: aws.String(tableName),
		Namespace: namespace,
		Encoder:   attributevalue.NewEncoder(),
		Decoder:   attributevalue.NewDecoder(),
		Now: func() time.Time {
			return time.Now().UTC()
		},
	}
	return
}

type Store struct {
	Client    *dynamodb.Client
	TableName *string
	Namespace string
	Encoder   *attributevalue.Encoder
	Decoder   *attributevalue.Decoder
	Now       func() time.Time
}

// Web Socket APIs can only stay open for two hours.
// So, delete old records after that time.
// https://docs.aws.amazon.com/apigateway/latest/developerguide/limits.html#apigateway-execution-service-websocket-limits-table
var maxConnectionDuration = (time.Hour * 2) + (time.Minute * 15)
var dateLayout = "20060102150405"

func newTopicConnectionRecord(topic, connectionID string, now time.Time) topicConnectionRecord {
	return topicConnectionRecord{
		PK:           fmt.Sprintf("topic/%s", topic),
		SK:           fmt.Sprintf("%s/%s", now.Format(dateLayout), connectionID),
		TTL:          now.Add(maxConnectionDuration).Unix(),
		Topic:        topic,
		ConnectionID: connectionID,
	}
}

type topicConnectionRecord struct {
	PK  string `dynamodbav:"pk"`
	SK  string `dynamodbav:"sk"`
	TTL int64  `dynamodbav:"ttl"`
	// Topic namespace that can be subscribed to.
	// /users
	// /users/123
	// /messages
	Topic string `dynamodbav:"t"`
	// ConnectionID that is subscribed to the topic.
	ConnectionID string `dynamodbav:"id"`
}

func (ddb *Store) Put(ctx context.Context, connectionID string, topic string) (err error) {
	item := newTopicConnectionRecord(topic, connectionID, ddb.Now())
	m, err := attributevalue.MarshalMap(item)
	if err != nil {
		return
	}
	_, err = ddb.Client.PutItem(ctx, &dynamodb.PutItemInput{
		Item:      m,
		TableName: ddb.TableName,
	})
	return
}

func (ddb *Store) BatchPut(ctx context.Context, connectionID string, topics []string) (err error) {
	writeRequests := make([]types.WriteRequest, len(topics))
	for i, topic := range topics {
		m, err := attributevalue.MarshalMap(newTopicConnectionRecord(topic, connectionID, ddb.Now()))
		if err != nil {
			return err
		}
		writeRequests[i] = types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: m,
			},
		}
	}
	bwi := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			*ddb.TableName: writeRequests,
		},
	}
	// Retry up to 5 times, over 6.2 seconds.
	bo := backoff.New(5)
	for {
		bwo, err := ddb.Client.BatchWriteItem(ctx, bwi)
		if err != nil {
			return err
		}
		if len(bwo.UnprocessedItems) == 0 {
			break
		}
		if err := bo(); err != nil {
			return err
		}
		bwi.RequestItems = bwo.UnprocessedItems
	}
	return nil
}

func (ddb *Store) Query(ctx context.Context, topicID string) (connectionIDs []string, err error) {
	p := dynamodb.NewQueryPaginator(ddb.Client, &dynamodb.QueryInput{
		TableName:              ddb.TableName,
		KeyConditionExpression: aws.String("#pk = :pk"),
		ExpressionAttributeNames: map[string]string{
			"#pk": "pk",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: topicID},
		},
	})
	var qo *dynamodb.QueryOutput
	for p.HasMorePages() {
		qo, err = p.NextPage(ctx)
		if err != nil {
			return
		}
		var page []topicConnectionRecord
		err = attributevalue.UnmarshalListOfMaps(qo.Items, &page)
		if err != nil {
			return
		}
		for _, record := range page {
			connectionIDs = append(connectionIDs, record.ConnectionID)
		}
	}
	return
}
