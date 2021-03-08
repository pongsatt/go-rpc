package messaging

// Msg struct
type Msg struct {
	Topic string
	Key   string
	Value []byte
	Reply string
	ID    string
}
