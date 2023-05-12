package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	awscdkapigateway "github.com/aws/aws-cdk-go/awscdkapigatewayv2alpha/v2"
	awscdkapigatewayintegrations "github.com/aws/aws-cdk-go/awscdkapigatewayv2integrationsalpha/v2"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type WebSocketStackProps struct {
	awscdk.StackProps
}

func NewWebSocketStack(scope constructs.Construct, id string, props *WebSocketStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// Store User ID to Connection Lookup.
	// Table...
	// pk          | sk           | connectionId     |
	// ------------|--------------|------------------|
	// user/123    | connectionId | <connection_id>  | # Each user's connections.
	// topic/ab12  | user/123     | NULL             | # Subscribers to an entity.
	// topic/ab12  | user/456     | NULL             |
	// topic/cd34  | user/123     | NULL             |

	subscriptionsTable := awsdynamodb.NewTable(stack, jsii.Ptr("Subscriptions"), &awsdynamodb.TableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.Ptr("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.Ptr("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		BillingMode:         awsdynamodb.BillingMode_PAY_PER_REQUEST,
		RemovalPolicy:       awscdk.RemovalPolicy_DESTROY,
		TimeToLiveAttribute: jsii.Ptr("ttl"),
	})

	bundlingOptions := &awslambdago.BundlingOptions{
		GoBuildFlags: &[]*string{jsii.Ptr(`-ldflags "-s -w" -tags lambda.norpc`)},
	}

	onConnect := awslambdago.NewGoFunction(stack, jsii.Ptr("OnConnect"), &awslambdago.GoFunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2(),
		Architecture: awslambda.Architecture_ARM_64(),
		MemorySize:   jsii.Ptr(1024.0),
		Entry:        jsii.Ptr("../connection/onconnect"),
		Bundling:     bundlingOptions,
		Environment: &map[string]*string{
			"CONNECTIONS_TABLE_NAME": subscriptionsTable.TableName(),
		},
	})
	subscriptionsTable.GrantWriteData(onConnect)

	onDisconnect := awslambdago.NewGoFunction(stack, jsii.Ptr("OnDisconnect"), &awslambdago.GoFunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2(),
		Architecture: awslambda.Architecture_ARM_64(),
		MemorySize:   jsii.Ptr(1024.0),
		Entry:        jsii.Ptr("../connection/ondisconnect"),
		Bundling:     bundlingOptions,
		Environment: &map[string]*string{
			"CONNECTIONS_TABLE_NAME": subscriptionsTable.TableName(),
		},
	})
	subscriptionsTable.GrantWriteData(onDisconnect)

	onDefault := awslambdago.NewGoFunction(stack, jsii.Ptr("OnDefault"), &awslambdago.GoFunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2(),
		Architecture: awslambda.Architecture_ARM_64(),
		MemorySize:   jsii.Ptr(1024.0),
		Entry:        jsii.Ptr("../connection/ondefault"),
		Bundling:     bundlingOptions,
		Environment: &map[string]*string{
			"CONNECTIONS_TABLE_NAME": subscriptionsTable.TableName(),
		},
	})

	webSocketAPI := awscdkapigateway.NewWebSocketApi(stack, jsii.Ptr("WebsocketApi"), &awscdkapigateway.WebSocketApiProps{
		ConnectRouteOptions: &awscdkapigateway.WebSocketRouteOptions{
			Integration:    awscdkapigatewayintegrations.NewWebSocketLambdaIntegration(jsii.Ptr("ConnectRoute"), onConnect),
			Authorizer:     awscdkapigateway.NewWebSocketNoneAuthorizer(),
			ReturnResponse: jsii.Ptr(true),
		},
		DefaultRouteOptions: &awscdkapigateway.WebSocketRouteOptions{
			Integration:    awscdkapigatewayintegrations.NewWebSocketLambdaIntegration(jsii.Ptr("DefaultRoute"), onDefault),
			ReturnResponse: jsii.Ptr(true),
		},
		DisconnectRouteOptions: &awscdkapigateway.WebSocketRouteOptions{
			Integration:    awscdkapigatewayintegrations.NewWebSocketLambdaIntegration(jsii.Ptr("DisconnectRoute"), onDisconnect),
			ReturnResponse: jsii.Ptr(true),
		},
	})
	awscdkapigateway.NewWebSocketStage(stack, jsii.Ptr("WebsocketApiStage"), &awscdkapigateway.WebSocketStageProps{
		AutoDeploy:   jsii.Ptr(true),
		StageName:    jsii.Ptr("wss"),
		WebSocketApi: webSocketAPI,
	})

	// Add /wss to the URL to access it.
	awscdk.NewCfnOutput(stack, jsii.String("url"), &awscdk.CfnOutputProps{
		ExportName: jsii.String("WebSocketAPI"),
		Value:      webSocketAPI.ApiEndpoint(),
	})

	//greetRoot := restAPI.Root().AddResource(jsii.String("greet"), &awsapigateway.ResourceOptions{})
	//greetPostIntegration := awsapigateway.NewLambdaIntegration(greetPost, &awsapigateway.LambdaIntegrationOptions{})
	//greetRoot.AddMethod(jsii.String("POST"), greetPostIntegration, &awsapigateway.MethodOptions{})

	// A queue you can use to send information to connected clients.
	sendQueue := awssqs.NewQueue(stack, jsii.Ptr("SendQueue"), &awssqs.QueueProps{
		DeadLetterQueue: &awssqs.DeadLetterQueue{
			MaxReceiveCount: jsii.Ptr(3.0),
			Queue: awssqs.NewQueue(stack, jsii.Ptr("SendQueueDLQ"), &awssqs.QueueProps{
				Encryption:    awssqs.QueueEncryption_SQS_MANAGED,
				EnforceSSL:    jsii.Ptr(true),
				RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
			}),
		},
		Encryption:    awssqs.QueueEncryption_SQS_MANAGED,
		EnforceSSL:    jsii.Ptr(true),
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})
	sender := awslambdago.NewGoFunction(stack, jsii.Ptr("Sender"), &awslambdago.GoFunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2(),
		Architecture: awslambda.Architecture_ARM_64(),
		MemorySize:   jsii.Ptr(1024.0),
		Entry:        jsii.Ptr("../sender"),
		Bundling:     bundlingOptions,
		Environment: &map[string]*string{
			"CONNECTIONS_TABLE_NAME": subscriptionsTable.TableName(),
		},
	})
	sender.AddEventSource(awslambdaeventsources.NewSqsEventSource(sendQueue, &awslambdaeventsources.SqsEventSourceProps{
		BatchSize:      jsii.Ptr(1.0),
		MaxConcurrency: jsii.Ptr(128.0),
	}))
	subscriptionsTable.GrantReadData(sender)

	return stack
}

func main() {
	defer jsii.Close()
	app := awscdk.NewApp(nil)
	NewWebSocketStack(app, "WebSocketStack", nil)
	app.Synth(nil)
}
