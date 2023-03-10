package services

type MessageService interface {
	Send(message Message) error
}

type Message interface {
	MessageType() string
}
