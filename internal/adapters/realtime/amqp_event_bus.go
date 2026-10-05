package realtime

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/url"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	amqpOperationTimeout      = 5 * time.Second
	amqpHeartbeatInterval     = 10 * time.Second
	amqpCloseTimeout          = time.Second
	amqpInitialReconnectDelay = 100 * time.Millisecond
	amqpMaximumReconnectDelay = 5 * time.Second
	amqpReconnectResetAfter   = 30 * time.Second
	amqpMessageTTL            = time.Minute
	amqpQueueMaxLength        = 1000
	amqpConsumerPrefetch      = 32
	amqpExchangeNameMaxLength = 255
)

// AMQPEventBus broadcasts through RabbitMQ or LavinMQ. Listen owns one
// connection and an exclusive, server-named queue for this process only.
type AMQPEventBus struct {
	url      string
	exchange string
	requests chan amqpPublishRequest
}

type amqpPublishRequest struct {
	ctx    context.Context
	body   []byte
	result chan error
}

func NewAMQPEventBus(address, exchange string) (*AMQPEventBus, error) {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "amqp" && parsed.Scheme != "amqps") {
		// Never include the URI or its parser error: both can contain credentials.
		return nil, errors.New("AMQP_URL must be a valid amqp:// or amqps:// URL")
	}
	if _, err := amqp.ParseURI(address); err != nil {
		return nil, errors.New("AMQP_URL contains invalid AMQP connection parameters")
	}
	if parsed.Scheme == "amqp" && parsed.Hostname() != "localhost" && parsed.Hostname() != "rabbitmq" {
		ip := net.ParseIP(parsed.Hostname())
		if ip == nil || !ip.IsLoopback() {
			return nil, errors.New("remote AMQP_URL connections require amqps://")
		}
	}
	if exchange == "" || len(exchange) > amqpExchangeNameMaxLength || strings.HasPrefix(exchange, "amq.") || strings.TrimSpace(exchange) != exchange {
		return nil, errors.New("AMQP_EXCHANGE must be a non-reserved name of 1 to 255 bytes")
	}
	return &AMQPEventBus{url: address, exchange: exchange, requests: make(chan amqpPublishRequest)}, nil
}

// Publish waits for broker confirmation, not delivery to a browser. Call only
// after committing the business data. An ambiguous failure is not retried.
func (bus *AMQPEventBus) Publish(ctx context.Context, event EventEnvelope) error {
	if err := validateEventEnvelope(event); err != nil {
		return fmt.Errorf("publishing AMQP event: %w", err)
	}
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encoding AMQP event: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, amqpOperationTimeout)
	defer cancel()
	request := amqpPublishRequest{ctx: ctx, body: body, result: make(chan error, 1)}
	select {
	case bus.requests <- request:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-request.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Listen runs once per API lifecycle. Every reconnect recreates the topology;
// replicas never need a registry, stable IDs, or addresses of their peers.
func (bus *AMQPEventBus) Listen(ctx context.Context, handler func(EventEnvelope)) error {
	if handler == nil {
		return errors.New("AMQP event handler is required")
	}
	delay := amqpInitialReconnectDelay
	for ctx.Err() == nil {
		started := time.Now()
		err := bus.listenOnce(ctx, handler)
		if ctx.Err() != nil {
			break
		}
		if time.Since(started) > amqpReconnectResetAfter {
			delay = amqpInitialReconnectDelay
		}
		wait := delay + time.Duration(rand.Int64N(int64(delay)))
		slog.Warn("AMQP realtime disconnected; reconnecting", "error", err, "retry_in", wait)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		delay = min(delay*2, amqpMaximumReconnectDelay)
	}
	return ctx.Err()
}

func (bus *AMQPEventBus) listenOnce(ctx context.Context, handler func(EventEnvelope)) error {
	session, err := openAMQPSession(ctx, bus.url)
	if err != nil {
		return err
	}
	defer session.close()
	if err := session.configure(bus.exchange); err != nil {
		return err
	}
	slog.Info("AMQP realtime subscribed", "exchange", bus.exchange)
	return bus.processEvents(ctx, session, handler)
}

func (bus *AMQPEventBus) processEvents(ctx context.Context, session *amqpSession, handler func(EventEnvelope)) error {
	closed := session.connection.NotifyClose(make(chan *amqp.Error, 1))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-closed:
			return amqp.ErrClosed
		case delivery, ok := <-session.deliveries:
			if !ok {
				return amqp.ErrClosed
			}
			if err := handleAMQPDelivery(delivery, handler); err != nil {
				return err
			}
		case request := <-bus.requests:
			if err := request.ctx.Err(); err != nil {
				request.result <- err
				continue
			}
			// one confirmed publish at a time; pipeline confirms if
			// broker round-trip latency limits throughput beyond the teaching scale.
			err := session.publishConfirmed(bus.exchange, request)
			request.result <- err
			if err != nil {
				return err
			}
		}
	}
}

func handleAMQPDelivery(delivery amqp.Delivery, handler func(EventEnvelope)) error {
	var event EventEnvelope
	if err := json.Unmarshal(delivery.Body, &event); err != nil || validateEventEnvelope(event) != nil {
		slog.Warn("discarding invalid AMQP realtime envelope")
		return delivery.Reject(false)
	}
	handler(event)
	return delivery.Ack(false)
}

// amqpSession owns the connection and channels for one subscription lifetime.
// Only the event loop uses its channels; cancellation may close the transport.
type amqpSession struct {
	connection       *amqp.Connection
	transport        net.Conn
	publisher        *amqp.Channel
	deliveries       <-chan amqp.Delivery
	stopCancellation func() bool
}

func openAMQPSession(ctx context.Context, address string) (*amqpSession, error) {
	session := &amqpSession{stopCancellation: func() bool { return true }}
	connection, err := amqp.DialConfig(address, amqp.Config{
		Heartbeat:       amqpHeartbeatInterval,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		Dial: func(network, address string) (net.Conn, error) {
			conn, err := (&net.Dialer{Timeout: amqpOperationTimeout}).DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			session.transport = conn
			session.stopCancellation = context.AfterFunc(ctx, func() { closeAMQPTransport(conn) })
			if err := conn.SetDeadline(time.Now().Add(amqpOperationTimeout)); err != nil {
				closeAMQPTransport(conn)
				return nil, err
			}
			return conn, nil
		},
	})
	if err != nil {
		if session.transport != nil {
			closeAMQPTransport(session.transport)
		}
		session.stopCancellation()
		return nil, fmt.Errorf("connecting to AMQP broker: %w", err)
	}
	session.connection = connection
	return session, nil
}

func (session *amqpSession) close() {
	defer session.stopCancellation()
	if err := session.connection.CloseDeadline(time.Now().Add(amqpCloseTimeout)); err != nil && !errors.Is(err, amqp.ErrClosed) {
		slog.Debug("closing AMQP connection", "error", err)
	}
}

func (session *amqpSession) configure(exchange string) error {
	// Bound channel/queue setup too; the library clears the handshake deadline.
	setupTimeout := time.AfterFunc(amqpOperationTimeout, func() { closeAMQPTransport(session.transport) })
	defer setupTimeout.Stop()
	var err error
	session.deliveries, err = session.subscribe(exchange)
	if err != nil {
		return err
	}
	session.publisher, err = session.connection.Channel()
	if err != nil {
		return err
	}
	return session.publisher.Confirm(false)
}

func (session *amqpSession) subscribe(exchange string) (<-chan amqp.Delivery, error) {
	consumer, err := session.connection.Channel()
	if err != nil {
		return nil, err
	}
	// The exchange survives disconnects; each replica owns a temporary queue.
	if err := consumer.ExchangeDeclare(exchange, amqp.ExchangeFanout, true, false, false, false, nil); err != nil {
		return nil, err
	}
	queue, err := consumer.QueueDeclare("", false, true, true, false, amqp.Table{
		"x-message-ttl": int32(amqpMessageTTL / time.Millisecond),
		"x-max-length":  int32(amqpQueueMaxLength),
	})
	if err != nil {
		return nil, err
	}
	if err := consumer.QueueBind(queue.Name, "", exchange, false, nil); err != nil {
		return nil, err
	}
	if err := consumer.Qos(amqpConsumerPrefetch, 0, false); err != nil {
		return nil, err
	}
	return consumer.Consume(queue.Name, "", false, true, false, false, nil)
}

func (session *amqpSession) publishConfirmed(exchange string, request amqpPublishRequest) error {
	// PublishWithContext cannot interrupt a blocked socket write. Closing the
	// transport on cancellation bounds I/O as well as the confirmation wait.
	stop := context.AfterFunc(request.ctx, func() { closeAMQPTransport(session.transport) })
	defer stop()
	confirmation, err := session.publisher.PublishWithDeferredConfirmWithContext(request.ctx, exchange, "", false, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Transient, Body: request.body,
	})
	if err != nil {
		return fmt.Errorf("publishing AMQP event: %w", err)
	}
	acknowledged, err := confirmation.WaitContext(request.ctx)
	if err != nil {
		return fmt.Errorf("confirming AMQP event: %w", err)
	}
	if !acknowledged {
		return errors.New("AMQP broker rejected realtime event")
	}
	return nil
}

func closeAMQPTransport(connection net.Conn) {
	if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		slog.Debug("closing AMQP transport", "error", err)
	}
}
