package config

type Backend interface {
	Get(filename string) (string, error)
	Set(filename, value string) error
}
