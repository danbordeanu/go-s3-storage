package configuration

import (
	"gopkg.in/yaml.v2"
	"log"
	"os"
)

type CSwagger struct {
	Version     string `yaml:"version"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	BasePath    string `yaml:"basePath"`
}

func (c *Configuration) LoadSwaggerConf() {
	yamlFile, err := os.ReadFile("swagger.yaml")
	if err != nil {
		log.Fatalf("Error opening swagger configuration file swagger.yaml: %v ", err)
	}
	err = yaml.Unmarshal(yamlFile, &c.Swagger)
	if err != nil {
		log.Fatalf("Unmarshal failed: %v", err)
	}
}
