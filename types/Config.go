package types

type Config struct {
	Applications Applications `json:"applications"`
	BosunAddress string       `json:"bosunAddress"`
}
