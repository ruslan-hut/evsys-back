package services

type Database interface {
	WriteLogMessage(data Data) error
	ReadSystemLog() (interface{}, error)
	ReadBackLog() (interface{}, error)
	AuthenticateUser(username string, password string) (interface{}, error)
}

type Data interface {
	DataType() string
}
