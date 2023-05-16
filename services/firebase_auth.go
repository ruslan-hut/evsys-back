package services

type FirebaseAuth interface {
	CheckToken(token string) error
}
