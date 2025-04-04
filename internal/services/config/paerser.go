package config

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/traefik/paerser/env"
	"github.com/traefik/paerser/file"
	"github.com/traefik/paerser/flag"
	"github.com/zekroTJA/shinpuru/internal/models"
)

const defaultConfigLoc = "./config.yaml"

type Paerser struct {
	cfg        *models.Config
	args       []string
	configFile string
}

func NewPaerser(args []string, configFile string) *Paerser {
	return &Paerser{
		args:       args,
		configFile: configFile,
	}
}

func (p *Paerser) Config() *models.Config {
	return p.cfg
}

func (p *Paerser) Parse() (err error) {
	cfg := models.DefaultConfig

	cfgFile := defaultConfigLoc
	if p.configFile != "" {
		cfgFile = p.configFile
	}
	if err = file.Decode(cfgFile, &cfg); err != nil && !os.IsNotExist(err) {
		return
	}

	godotenv.Load()
	if err = env.Decode(os.Environ(), "SP_", &cfg); err != nil {
		return
	}

	args := os.Args[1:]
	if p.args != nil {
		args = p.args
	}
	if err = flag.Decode(args, &cfg); err != nil {
		return
	}

	p.cfg = &cfg

	return
}
