package updog

type Config struct {
	Applications map[string]*Application `json:"applications"`
	BosunAddress string                  `json:"bosunAddress"`
}
