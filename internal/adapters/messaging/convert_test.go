package messaging

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"digital.vasic.messaging/pkg/broker"

	"dev.helix.agent/internal/messaging"
)

func TestInternalToGenericMessage(t *testing.T) {
	t.Parallel()
	internal := messaging.NewMessage("test-type", []byte("test payload"))
	internal.ID = "msg-123"
	internal.TraceID = "trace-456"
	internal.CorrelationID = "corr-789"
	internal.ReplyTo = "reply-queue"
	internal.SetHeader("custom-header", "custom-value")

	generic := InternalToGenericMessage(internal)

	assert.Equal(t, internal.ID, generic.ID)
	assert.Equal(t, internal.Type, generic.Topic)
	assert.Equal(t, internal.Payload, generic.Value)
	assert.Equal(t, internal.Timestamp, generic.Timestamp)
	assert.Equal(t, "trace-456", generic.GetHeader("x-trace-id"))
	assert.Equal(t, "corr-789", generic.GetHeader("x-correlation-id"))
	assert.Equal(t, "reply-queue", generic.GetHeader("x-reply-to"))
	assert.Equal(t, "custom-value", generic.GetHeader("custom-header"))
}

func TestInternalToGenericMessage_Nil(t *testing.T) {
	t.Parallel()
	result := InternalToGenericMessage(nil)
	assert.Nil(t, result)
}

func TestGenericToInternalMessage(t *testing.T) {
	t.Parallel()
	generic := broker.NewMessage("test-topic", []byte("test payload"))
	generic.ID = "msg-123"
	generic.SetHeader("x-trace-id", "trace-456")
	generic.SetHeader("x-correlation-id", "corr-789")
	generic.SetHeader("x-reply-to", "reply-queue")
	generic.SetHeader("custom-header", "custom-value")

	internal := GenericToInternalMessage(generic)

	assert.Equal(t, generic.ID, internal.ID)
	assert.Equal(t, generic.Topic, internal.Type)
	assert.Equal(t, generic.Value, internal.Payload)
	assert.Equal(t, generic.Timestamp, internal.Timestamp)
	assert.Equal(t, "trace-456", internal.TraceID)
	assert.Equal(t, "corr-789", internal.CorrelationID)
	assert.Equal(t, "reply-queue", internal.ReplyTo)
	assert.Equal(t, "custom-value", internal.GetHeader("custom-header"))
}

func TestGenericToInternalMessage_Nil(t *testing.T) {
	t.Parallel()
	result := GenericToInternalMessage(nil)
	assert.Nil(t, result)
}

func TestBrokerTypeConversion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		internal messaging.BrokerType
		generic  broker.BrokerType
	}{
		{messaging.BrokerTypeKafka, broker.BrokerTypeKafka},
		{messaging.BrokerTypeRabbitMQ, broker.BrokerTypeRabbitMQ},
		{messaging.BrokerTypeInMemory, broker.BrokerTypeInMemory},
	}

	for _, tt := range tests {
		t.Run(string(tt.internal), func(t *testing.T) {
			t.Parallel()
			// Internal to Generic
			assert.Equal(t, tt.generic, InternalToGenericBrokerType(tt.internal))

			// Generic to Internal
			assert.Equal(t, tt.internal, GenericToInternalBrokerType(tt.generic))
		})
	}
}

func TestBrokerTypeConversion_Unknown(t *testing.T) {
	t.Parallel()
	// Unknown internal type should map to InMemory
	result := InternalToGenericBrokerType(messaging.BrokerType("unknown"))
	assert.Equal(t, broker.BrokerTypeInMemory, result)

	// Unknown generic type should map to InMemory
	result2 := GenericToInternalBrokerType(broker.BrokerType("unknown"))
	assert.Equal(t, messaging.BrokerTypeInMemory, result2)
}

func TestErrorConversion(t *testing.T) {
	t.Parallel()
	internalErr := messaging.NewBrokerError(
		messaging.ErrCodeConnectionFailed,
		"test error message",
		nil,
	)

	genericErr := InternalToGenericError(internalErr)
	assert.Equal(t, broker.ErrorCode(internalErr.Code), genericErr.Code)
	assert.Equal(t, internalErr.Message, genericErr.Message)

	// Convert back
	backToInternal := GenericToInternalError(genericErr)
	assert.Equal(t, internalErr.Code, backToInternal.Code)
	assert.Equal(t, internalErr.Message, backToInternal.Message)
}

func TestErrorConversion_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, InternalToGenericError(nil))
	assert.Nil(t, GenericToInternalError(nil))
}

func TestMessageBatch(t *testing.T) {
	t.Parallel()
	internal := []*messaging.Message{
		messaging.NewMessage("type1", []byte("payload1")),
		messaging.NewMessage("type2", []byte("payload2")),
	}

	generic := MessageBatch(internal)
	assert.Len(t, generic, 2)
	assert.Equal(t, internal[0].Payload, generic[0].Value)
	assert.Equal(t, internal[1].Payload, generic[1].Value)
}

func TestInternalMessageBatch(t *testing.T) {
	t.Parallel()
	generic := []*broker.Message{
		broker.NewMessage("topic1", []byte("payload1")),
		broker.NewMessage("topic2", []byte("payload2")),
	}

	internal := InternalMessageBatch(generic)
	assert.Len(t, internal, 2)
	assert.Equal(t, generic[0].Value, internal[0].Payload)
	assert.Equal(t, generic[1].Value, internal[1].Payload)
}

func TestTimestampHelpers(t *testing.T) {
	t.Parallel()
	now := time.Now()
	millis := TimestampToMillis(now)
	recovered := MillisToTimestamp(millis)

	// Due to precision loss, compare at millisecond level
	assert.Equal(t, now.UnixMilli(), recovered.UnixMilli())
}
