package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type ServiceNode struct {
	Name          string            `yaml:"name"`
	DependsOn     []string          `yaml:"dependsOn"`
	LabelSelector map[string]string `yaml:"labelSelector,omitempty"`
}

type ServiceGraphConfig struct {
	Services []ServiceNode `yaml:"services"`
	Entry    string        `yaml:"entry"`
}

type PrometheusConfig struct {
	URL                string `yaml:"url"`
	NodeRTTQuery       string `yaml:"nodeRTTQuery"`
	NodeDropRateQuery  string `yaml:"nodeDropRateQuery"`
	NodeBandwidthQuery string `yaml:"nodeBandwidthQuery"`
	SampleWindow       string `yaml:"sampleWindow"`
}

type ScoringWeights struct {
	PathLengthWeight   float64 `yaml:"pathLengthWeight"`
	PodCountWeight     float64 `yaml:"podCountWeight"`
	ServiceEdgesWeight float64 `yaml:"serviceEdgesWeight"`
	RPSWeight          float64 `yaml:"rpsWeight"`

	NetLatencyWeight   float64 `yaml:"netLatencyWeight"`
	NetDropWeight      float64 `yaml:"netDropWeight"`
	NetBandwidthWeight float64 `yaml:"netBandwidthWeight"`
}

type AffinityConfig struct {
	TopPaths          int     `yaml:"topPaths"`
	MinAffinityWeight int     `yaml:"minAffinityWeight"`
	MaxAffinityWeight int     `yaml:"maxAffinityWeight"`
	BadLatencyMs      float64 `yaml:"badLatencyMs"`
	BadDropRate       float64 `yaml:"badDropRate"`
}

type Config struct {
	NamespaceSelector []string           `yaml:"namespaceSelector"`
	Graph             ServiceGraphConfig `yaml:"graph"`
	Prometheus        PrometheusConfig   `yaml:"prometheus"`
	Scoring           ScoringWeights     `yaml:"scoring"`
	Affinity          AffinityConfig     `yaml:"affinity"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var c Config
	if err := yaml.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}
