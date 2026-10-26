package message

import (
	"ecommerce/order-service/config"
	"ecommerce/order-service/internal/core/domain/entity"
	"ecommerce/order-service/utils"
	"encoding/json"
	"fmt"

	"github.com/labstack/gommon/log"
	"github.com/streadway/amqp"
)

type PublishRabbitMQInterface interface {
	PublishOrderToQueue(order entity.OrderEntity) error
	PublishSendEmailUpdateStatus(email, message, queuename string, userID int64) error
	PublishDeleteOrderFromQueue(orderID int64) error
	PublishSendPushNotifUpdateStatus(message, queuename string, userID int64) error
	PublishUpdateStatus(queuename string, orderID int64, status string) error
}

type PublishRabbitMQ struct {
	cfg *config.Config
}

// PublishUpdateStatus implements PublishRabbitMQInterface.
func (p *PublishRabbitMQ) PublishUpdateStatus(queuename string, orderID int64, status string) error {
	conn, err := p.cfg.NewRabbitMQ()
	if err != nil {
		log.Errorf("[PublishUpdateStatus-1] Failed to connect to RabbitMQ: %v", err)
		return err
	}

	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Errorf("[PublishUpdateStatus-2] Failed to open a channel: %v", err)
		return err
	}

	defer ch.Close()

	queue, err := ch.QueueDeclare(
		queuename,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Errorf("[PublishUpdateStatus-3] Failed to declare a queue: %v", err)
		return err
	}

	orderStatus := map[string]string{
		"orderID": fmt.Sprintf("%d", orderID),
		"status":  status,
	}

	body, err := json.Marshal(orderStatus)
	if err != nil {
		log.Errorf("[PublishUpdateStatus-4] Failed to marshal JSON: %v", err)
		return err
	}

	return ch.Publish(
		"",
		queue.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// PublishSendPushNotifUpdateStatus implements PublishRabbitMQInterface.
func (p *PublishRabbitMQ) PublishSendPushNotifUpdateStatus(message string, queuename string, userID int64) error {
	conn, err := p.cfg.NewRabbitMQ()
	if err != nil {
		log.Errorf("[PublishSendEmailUpdateStatus-1] Failed to connect to RabbitMQ: %v", err)
		return err
	}

	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Errorf("[PublishSendEmailUpdateStatus-2] Failed to open a channel: %v", err)
		return err
	}

	defer ch.Close()

	queue, err := ch.QueueDeclare(
		queuename,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Errorf("[PublishSendEmailUpdateStatus-3] Failed to declare a queue: %v", err)
		return err
	}

	notifType := "EMAIL"
	if queuename == utils.PUSH_NOTIF {
		notifType = "PUSH"
	}

	notification := map[string]interface{}{
		"receiver_email":    "",
		"message":           message,
		"subject":           "Update Status Order",
		"type":              "UPDATE_STATUS",
		"receiver_id":       userID,
		"notification_type": notifType,
	}

	body, err := json.Marshal(notification)
	if err != nil {
		log.Errorf("[PublishSendEmailUpdateStatus-4] Failed to marshal JSON: %v", err)
		return err
	}

	return ch.Publish(
		"",
		queue.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// PublishDeleteOrderFromQueue implements PublishRabbitMQInterface.
func (p *PublishRabbitMQ) PublishDeleteOrderFromQueue(orderID int64) error {
	conn, err := p.cfg.NewRabbitMQ()
	if err != nil {
		log.Errorf("[PublishDeleteOrderFromQueue-1] Failed to connect to RabbitMQ: %v", err)
		return err
	}

	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Errorf("[PublishDeleteOrderFromQueue-2] Failed to open a channel: %v", err)
		return err
	}

	defer ch.Close()

	queue, err := ch.QueueDeclare(
		p.cfg.PublisherName.PublisherDeleteOrder,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Errorf("[PublishDeleteOrderFromQueue-3] Failed to declare a queue: %v", err)
		return err
	}

	order := map[string]string{
		"orderID": fmt.Sprintf("%d", orderID),
	}

	body, err := json.Marshal(order)
	if err != nil {
		log.Errorf("[PublishDeleteOrderFromQueue-4] Failed to marshal JSON: %v", err)
		return err
	}

	return ch.Publish(
		"",
		queue.Name,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

// PublishSendEmailUpdateStatus implements PublishRabbitMQInterface.
func (p *PublishRabbitMQ) PublishSendEmailUpdateStatus(email, message, queuename string, userID int64) error {
	conn, err := p.cfg.NewRabbitMQ()
	if err != nil {
		log.Errorf("[PublishSendEmailUpdateStatus-1] Failed to connect to RabbitMQ: %v", err)
		return err
	}

	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Errorf("[PublishSendEmailUpdateStatus-2] Failed to open a channel: %v", err)
		return err
	}

	defer ch.Close()

	queue, err := ch.QueueDeclare(
		queuename,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Errorf("[PublishSendEmailUpdateStatus-3] Failed to declare a queue: %v", err)
		return err
	}

	notifType := "EMAIL"
	if queuename == utils.PUSH_NOTIF {
		notifType = "PUSH"
	}

	notification := map[string]interface{}{
		"receiver_email":    email,
		"message":           message,
		"subject":           "Update Status Order",
		"type":              "UPDATE_STATUS",
		"receiver_id":       userID,
		"notification_type": notifType,
	}

	body, err := json.Marshal(notification)
	if err != nil {
		log.Errorf("[PublishSendEmailUpdateStatus-4] Failed to marshal JSON: %v", err)
		return err
	}

	return ch.Publish(
		"",
		queue.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// PublishOrderToQueue implements PublishRabbitMQInterface.
func (p *PublishRabbitMQ) PublishOrderToQueue(order entity.OrderEntity) error {
	conn, err := p.cfg.NewRabbitMQ()
	if err != nil {
		log.Errorf("[PublishOrderToQueue-1] Failed to connect to RabbitMQ: %v", err)
		return err
	}

	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Errorf("[PublishOrderToQueue-2] Failed to open a channel: %v", err)
		return err
	}

	defer ch.Close()

	q, err := ch.QueueDeclare(
		p.cfg.PublisherName.OrderPublish,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Errorf("[PublishOrderToQueue-3] Failed to declare queue: %v", err)
		return err
	}

	data, _ := json.Marshal(order)
	err = ch.Publish(
		"",
		q.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        data,
		},
	)
	if err != nil {
		log.Errorf("[PublishOrderToQueue-4] Failed to publish message: %v", err)
		return err
	}

	return nil
}

func NewPublisherRabbitMQ(cfg *config.Config) PublishRabbitMQInterface {
	return &PublishRabbitMQ{cfg: cfg}
}
