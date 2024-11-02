package authenticator

type FirebaseAuth interface {
	CheckToken(token string) (string, error)
}
