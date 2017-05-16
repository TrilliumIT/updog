package types

type Config struct {
	Applications    Applications `json:"applications"`
	OpenTSDBAddress string       `json:"opentsdb_address"`
}
