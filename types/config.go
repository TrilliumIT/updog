package types

//Config represents updogs configuration yaml
type Config struct {
	Applications    *Applications `json:"applications"`
	OpenTSDBAddress string        `json:"opentsdb_address"`
}
