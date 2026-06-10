package nats_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wrany/libs/eventbus"
	natstransport "github.com/wrany/tracking-worker/internal/transport/nats"
	"github.com/wrany/tracking-worker/internal/usecase"
)

// ---- fakes ----

type fakeProcessor struct {
	result usecase.ProcessingResult
}

func (f *fakeProcessor) Process(_ context.Context, _ usecase.ProcessingInput) usecase.ProcessingResult {
	return f.result
}

type fakeMessage struct {
	data   []byte
	ackErr error
	nakErr error
	acked  bool
	nacked bool
}

func (m *fakeMessage) Subject() string              { return "location.events.v1" }
func (m *fakeMessage) Data() []byte                { return m.data }
func (m *fakeMessage) Headers() map[string][]string { return nil }
func (m *fakeMessage) Ack() error                  { m.acked = true; return m.ackErr }
func (m *fakeMessage) Nak() error                  { m.nacked = true; return m.nakErr }

var _ eventbus.Message = (*fakeMessage)(nil)

// captureProcessor records the ProcessingInput it received.
type captureProcessor struct {
	captured usecase.ProcessingInput
}

func (c *captureProcessor) Process(_ context.Context, input usecase.ProcessingInput) usecase.ProcessingResult {
	c.captured = input
	return usecase.ProcessingResult{Action: usecase.ActionAck}
}

// ---- tests ----

func TestHandle_ActionAck_CallsMsgAck(t *testing.T) {
	proc := &fakeProcessor{result: usecase.ProcessingResult{Action: usecase.ActionAck}}
	msg := &fakeMessage{data: []byte("payload")}

	lc := natstransport.NewLocationConsumerForTest(proc)
	err := lc.HandleForTest(context.Background(), msg)

	require.NoError(t, err)
	assert.True(t, msg.acked, "Ack must be called on ActionAck")
	assert.False(t, msg.nacked)
}

func TestHandle_ActionNak_CallsMsgNak(t *testing.T) {
	proc := &fakeProcessor{result: usecase.ProcessingResult{Action: usecase.ActionNak}}
	msg := &fakeMessage{data: []byte("payload")}

	lc := natstransport.NewLocationConsumerForTest(proc)
	err := lc.HandleForTest(context.Background(), msg)

	require.NoError(t, err)
	assert.True(t, msg.nacked, "Nak must be called on ActionNak")
	assert.False(t, msg.acked)
}

func TestHandle_AckError_ReturnsError(t *testing.T) {
	proc := &fakeProcessor{result: usecase.ProcessingResult{Action: usecase.ActionAck}}
	msg := &fakeMessage{data: []byte("x"), ackErr: errors.New("ack failed")}

	lc := natstransport.NewLocationConsumerForTest(proc)
	err := lc.HandleForTest(context.Background(), msg)

	assert.Error(t, err)
}

func TestHandle_PassesMsgDataToProcessor(t *testing.T) {
	cap := &captureProcessor{}
	msg := &fakeMessage{data: []byte("raw-bytes")}

	lc := natstransport.NewLocationConsumerForTest(cap)
	_ = lc.HandleForTest(context.Background(), msg)

	assert.Equal(t, []byte("raw-bytes"), cap.captured.Data,
		"transport must pass msg.Data() verbatim to processor")
}
