package src

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"fmt"
)

func TestParseConfig(t *testing.T) {
	config := ClientConfig{}
	ParseClientConfig(&config, "./config_sample.json")
	assert.Equal(t, "2.2.2.2:444", config.Target)
	assert.Equal(t, 15, config.Conn)
	fmt.Println(config)
	config2 := ServerConfig{}
	ParseServerConfig(&config2, "./config_sample.json")
	assert.Equal(t, "2.2.2.2:444", config2.Target)
	assert.Equal(t, true, config2.Pprof)
}
