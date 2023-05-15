# WebSocket API

## Tasks

### cdk-deploy

dir: ./cdk

```sh
cdk deploy
```

### cdk-deploy-hotswap

dir: ./cdk

```sh
cdk deploy --hotswap
```

### send-message

Inputs: QUEUE_URL, CONNECTION_ID, MESSAGE_BODY

```sh
aws sqs send-message --queue-url $QUEUE_URL --message-attributes "connectionId={StringValue=$CONNECTION_ID,DataType=String}" --message-body $MESSAGE_BODY
```

### connect

Connect to the WebSocket API Endpoint, e.g. wss://aaaaaaaa.execute-api.eu-west-2.amazonaws.com/wss/

Requires the wscat tool, see https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-how-to-call-websocket-api-wscat.html

Inputs: ADDRESS

```sh
wscat -c $ADDRESS
```

### logs-connect

```
saw get --region eu-west-2 --fuzzy WebSocketStack-OnConnect
```

### logs-sender

```
saw get --region eu-west-2 --fuzzy WebSocketStack-Sender
```
